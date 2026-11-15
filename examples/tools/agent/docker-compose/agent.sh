#!/bin/sh

if [ -z "${INFERENCE_GATEWAY_URL}" ]; then
  echo "Error: INFERENCE_GATEWAY_URL is not set."
  echo "Please set the INFERENCE_GATEWAY_URL environment variable."
  exit 1
fi

if [ -z "${MODEL}" ]; then
  echo "Error: MODEL is not set."
  echo "Please set the MODEL environment variable."
  exit 1
fi

AGENT_INTERVAL=${AGENT_INTERVAL:-30}
input_data="Tell me about the current weather in Tokyo and the latest news on renewable energy."

echo "Starting agent with model: ${MODEL}"
echo "Using input: ${input_data}"

# Function to handle tool calls
handle_tool_call() {
  local tool_name=$1
  local tool_args=$2
  local tool_id=$3

  echo "Handling tool call: ${tool_name}"
  echo "Arguments: ${tool_args}"
  
  if [ "$tool_name" = "web_search" ]; then
    # Extract query from tool arguments
    query=$(echo "$tool_args" | jq -r '.query')
    echo "Search query: ${query}"
    
    # Load mock search data from the JSON file
    case "$query" in
      *weather*Tokyo*)
        result=$(jq -r '.weather.locations."Tokyo, Japan"' /app/mocks/web_search.json)
        ;;
      *renewable*energy*)
        result=$(jq -r '.search.queries."renewable energy"' /app/mocks/web_search.json)
        ;;
      *)
        result="{\"error\": \"No results found for query: ${query}\"}"
        ;;
    esac
    
    echo "Search results: ${result}"
    return_result=$(echo "$result" | jq -c '.')
  elif [ "$tool_name" = "get_weather" ]; then
    # Extract location from tool arguments
    location=$(echo "$tool_args" | jq -r '.location')
    echo "Weather location: ${location}"
    
    # Load mock weather data from the JSON file
    case "$location" in
      *Tokyo*)
        result=$(jq -r '.weather.locations."Tokyo, Japan"' /app/mocks/web_search.json)
        ;;
      *)
        result="{\"error\": \"Weather data not available for location: ${location}\"}"
        ;;
    esac
    
    echo "Weather results: ${result}"
    return_result=$(echo "$result" | jq -c '.')
  else
    return_result="{\"error\": \"Unknown tool: ${tool_name}\"}"
  fi
  
  echo "Tool result: ${return_result}"
  return 0
}

while true; do
  echo "$(date) - Starting new agent interaction"
  
  # Step 1: Send initial request to the inference gateway
  echo "Sending initial request to inference gateway..."
  response=$(curl -s -X POST "${INFERENCE_GATEWAY_URL}/chat/completions" \
    -H "Content-Type: application/json" \
    -d '{
      "model": "'"${MODEL}"'",
      "messages": [
        {
          "role": "system",
          "content": "You are a helpful assistant."
        },
        {
          "role": "user",
          "content": "'"${input_data}"'"
        }
      ],
      "tools": [
        {
          "type": "function",
          "function": {
            "name": "web_search",
            "description": "Searching the web for information",
            "parameters": {
              "type": "object",
              "properties": {
                "query": {
                  "type": "string",
                  "description": "The search query"
                }
              },
              "required": ["query"]
            }
          }
        },
        {
          "type": "function",
          "function": {
            "name": "get_weather",
            "description": "Get current weather information",
            "parameters": {
              "type": "object",
              "properties": {
                "location": {
                  "type": "string",
                  "description": "The city and optional country"
                }
              },
              "required": ["location"]
            }
          }
        }
      ]
    }')
  
  if [ $? -ne 0 ]; then
    echo "Error: Failed to call inference gateway."
    sleep "$AGENT_INTERVAL"
    continue
  fi
  
  echo "Response received from inference gateway"
  
  # Step 2: Check if the response contains tool calls
  finish_reason=$(echo "$response" | jq -r '.choices[0].finish_reason')
  
  # Create messages array starting with system and user messages
  messages='[
    {"role": "system", "content": "You are a helpful assistant."},
    {"role": "user", "content": "'"${input_data}"'"}
  ]'
  
  # Add assistant message with the response
  assistant_message=$(echo "$response" | jq '.choices[0].message')
  messages=$(echo "$messages" | jq '. += ['"$assistant_message"']')
  
  # Check if we need to execute any tool calls
  if [ "$finish_reason" = "tool_calls" ]; then
    echo "Tool calls detected"
    
    # Extract tool calls from the response
    tool_calls=$(echo "$response" | jq '.choices[0].message.tool_calls')
    num_tools=$(echo "$tool_calls" | jq 'length')
    
    echo "Number of tool calls: $num_tools"
    
    # Process each tool call
    i=0
    while [ "$i" -lt "$num_tools" ]; do
      tool_call=$(echo "$tool_calls" | jq ".[$i]")
      tool_id=$(echo "$tool_call" | jq -r '.id')
      tool_type=$(echo "$tool_call" | jq -r '.type')
      
      if [ "$tool_type" = "function" ]; then
        tool_name=$(echo "$tool_call" | jq -r '.function.name')
        tool_args=$(echo "$tool_call" | jq -r '.function.arguments')
        
        echo "Executing tool call $i: $tool_name (ID: $tool_id)"
        
        # Call the function to handle the tool call
        handle_tool_call "$tool_name" "$tool_args" "$tool_id"
        
        # Convert JSON result to a string and properly escape it for JSON
        return_result_escaped=$(echo "$return_result" | jq -Rs .)
        # Remove the outer quotes that jq adds
        return_result_escaped="${return_result_escaped%\"}"
        return_result_escaped="${return_result_escaped#\"}"
        
        # Add the tool result to messages as a properly escaped string
        tool_message="{\"role\": \"tool\", \"tool_call_id\": \"$tool_id\", \"content\": \"$return_result_escaped\"}"
        messages=$(echo "$messages" | jq '. += ['"$tool_message"']')
      fi
      
      i=$((i + 1))
    done
    
    # Step 3: Send follow-up request with tool results
    echo "Sending follow-up request with tool results..."
    # Convert messages to a JSON string that's properly escaped for curl
    messages_json=$(echo "$messages" | jq -c .)
    
    # Remove the 'reasoning' field from any assistant messages to avoid errors
    messages_json=$(echo "$messages_json" | jq '[.[] | if .role == "assistant" then del(.reasoning) else . end]')
    
    final_response=$(curl -s -X POST "${INFERENCE_GATEWAY_URL}/chat/completions" \
      -H "Content-Type: application/json" \
      -d "{
        \"model\": \"${MODEL}\",
        \"messages\": ${messages_json}
      }")
    
    # Print the final content for ease of reading
    echo "Final content:"
    echo "$final_response" | jq -r '.choices[0].message.content'
  else
    echo "No tool calls needed. Final response:"
    echo "$response" | jq -r '.choices[0].message.content'
  fi
  
  echo "----------------------------------------"
  echo "Waiting $AGENT_INTERVAL seconds before next interaction..."
  sleep "$AGENT_INTERVAL"
done

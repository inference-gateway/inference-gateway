package guardrails

# Block requests containing credit card numbers in the body.
# This is a simple example - in production, use the built-in detectors
# which include Luhn validation.
main := result if {
    contains(input.request.body, "4111")
    result := {"action": "block", "message": "request contains sensitive payment information"}
} else := {"action": "allow"}

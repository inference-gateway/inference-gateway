package guardrails

# A2A demo guardrail: refuse every call addressed to the mock-b agent before
# the relay forwards it. On /a2a the policy sees the raw JSON-RPC body of the
# call as input.request.body, and the agent a call targets is named in
# params.tenant (SendMessage, SendStreamingMessage) or in the "mock-b:"-prefixed
# task id of a follow-up call (GetTask). ListTasks without a tenant names no
# agent, so it relays as always.
#
# An input the rules do not match - any other path, phase or agent, a body that
# is not valid JSON - leaves main undefined, which the evaluator counts as
# allow. Keep this conditional main mutually exclusive with any other
# conditional main in this directory.
main := {"action": "block", "message": "agent mock-b is off limits on this gateway"} if {
    input.path == "/a2a"
    input.phase == "pre_call"
    targets_mock_b
}

targets_mock_b if {
    rpc := json.unmarshal(input.request.body)
    rpc.params.tenant == "mock-b"
}

targets_mock_b if {
    rpc := json.unmarshal(input.request.body)
    startswith(rpc.params.id, "mock-b:")
}

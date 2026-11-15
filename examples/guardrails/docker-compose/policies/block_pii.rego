package guardrails

# Block requests containing credit card numbers in the body.
# This is a simple example - in production, use the built-in detectors
# which include Luhn validation.
main := {"action": "block", "message": "request contains sensitive payment information"} if {
    contains(input.request.body, "4111")
}

package guardrails

default main := {"action": "allow"}

# https://accounts.google.com is one issuer for every Google account, so passing
# authentication only proves that some Google identity minted a token for our
# audience. This rule narrows that to our own service account.
main := {"action": "block", "message": "caller is not an allowed service account"} if {
	input.identity
	not allowed_emails[input.identity.email]
}

# Filled in by the setup step in README.md.
allowed_emails := {"inference-gateway-client@YOUR_PROJECT_ID.iam.gserviceaccount.com"}

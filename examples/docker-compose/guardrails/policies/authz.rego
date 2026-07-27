package guardrails

# Authorization example: control what an authenticated user is allowed to do.
#
# input.identity holds the verified OIDC claims of the caller (sub, email,
# groups, roles, ...). It is populated by the auth middleware, so this policy
# only has teeth when AUTH_ENABLE=true. When auth is off, input.identity is
# undefined and every rule below simply does not match (default allow).
#
# NOTE: `default main` lives in allow_all.rego; each conditional `main` body
# here overrides it. Keep conditional bodies mutually exclusive - if two match
# the same request with different values, OPA raises a complete-rule conflict.

# Restrict an expensive model to a single group.
# Members outside "ml-eng" are blocked from openai/gpt-4o.
main := {"action": "block", "message": "openai/gpt-4o is restricted to the ml-eng group"} if {
	input.request.model == "openai/gpt-4o"
	input.identity # only enforce for authenticated callers
	not group_allowed
}

group_allowed if {
	some g in input.identity.groups
	g == "ml-eng"
}

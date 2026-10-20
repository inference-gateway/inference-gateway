package core

import (
	_ "embed"
	"encoding/json"
	"sync"
)

// deprecatedStatus is the models.dev lifecycle status of a model the vendor
// has retired from its catalog but may still serve.
const deprecatedStatus = "deprecated"

// communityStatusesJSON is the fallback lifecycle-status table synced from the
// community-maintained models.dev dataset via `task status:sync`
// (internal/communitygen), keyed by "<provider>/<model>".
//
//go:embed community_statuses.json
var communityStatusesJSON []byte

// communityStatuses lazily parses the embedded table once. The file is
// generated and committed, so a parse failure is a build defect, not a runtime
// condition; it degrades to an empty table (no model counts as deprecated).
var communityStatuses = sync.OnceValue(func() map[string]string {
	table := make(map[string]string)
	_ = json.Unmarshal(communityStatusesJSON, &table)
	return table
})

// IsDeprecated reports whether the community status table marks the gateway
// model ID as deprecated. Unknown models are not deprecated.
func IsDeprecated(id string) bool {
	table := communityStatuses()
	for _, key := range communityLookupKeys(id) {
		if status, ok := table[key]; ok {
			return status == deprecatedStatus
		}
	}
	return false
}

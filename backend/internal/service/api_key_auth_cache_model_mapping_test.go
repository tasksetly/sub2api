package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAPIKeyAuthSnapshotGroupModelMappingRoundtrip(t *testing.T) {
	groupID := int64(91)
	apiKey := &APIKey{
		ID: 92, UserID: 46, GroupID: &groupID, Key: "sk-group-model-mapping", Status: StatusActive,
		User: &User{ID: 46, Status: StatusActive},
		Group: &Group{
			ID: groupID, Name: "group-model-mapping", Platform: PlatformOpenAI, Status: StatusActive,
			ModelMapping: map[string]string{"a": "b"},
		},
	}
	svc := &APIKeyService{}

	payload, err := json.Marshal(&APIKeyAuthCacheEntry{Snapshot: svc.snapshotFromAPIKey(context.Background(), apiKey)})
	require.NoError(t, err)
	var cached APIKeyAuthCacheEntry
	require.NoError(t, json.Unmarshal(payload, &cached))

	materialized, used, err := svc.applyAuthCacheEntry(apiKey.Key, &cached)
	require.NoError(t, err)
	require.True(t, used)
	require.NotNil(t, materialized.Group)
	require.Equal(t, map[string]string{"a": "b"}, materialized.Group.ModelMapping)
	got, matched := materialized.Group.ResolveGroupModel("a")
	require.True(t, matched)
	require.Equal(t, "b", got)
}

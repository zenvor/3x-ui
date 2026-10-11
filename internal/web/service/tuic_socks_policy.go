package service

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/mhsanaei/3x-ui/v3/internal/tuic"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// TUIC owns per-client accounting. Authenticate the relay's real email for
// routing, but disable Xray's user counters to avoid charging the same bytes twice.
func ensureTuicSocksPolicy(cfg *xray.Config) (uint32, error) {
	policy := map[string]json.RawMessage{}
	if len(cfg.Policy) > 0 {
		if err := json.Unmarshal(cfg.Policy, &policy); err != nil {
			return 0, err
		}
	}
	if policy == nil {
		policy = map[string]json.RawMessage{}
	}
	levels := map[string]json.RawMessage{}
	if raw := policy["levels"]; len(raw) > 0 {
		if err := json.Unmarshal(raw, &levels); err != nil {
			return 0, err
		}
	}
	if levels == nil {
		levels = map[string]json.RawMessage{}
	}
	used := map[uint32]bool{}
	for key := range levels {
		if n, err := strconv.ParseUint(key, 10, 32); err == nil {
			used[uint32(n)] = true
		}
	}
	for _, inbound := range cfg.InboundConfigs {
		var settings struct {
			UserLevel uint32 `json:"userLevel"`
			Clients   []struct {
				Level uint32 `json:"level"`
			} `json:"clients"`
		}
		if json.Unmarshal(inbound.Settings, &settings) == nil {
			used[settings.UserLevel] = true
			for _, client := range settings.Clients {
				used[client.Level] = true
			}
		}
	}
	level := ^uint32(0)
	for used[level] {
		if level == 0 {
			return 0, fmt.Errorf("no available TUIC relay policy level")
		}
		level--
	}
	// Preserve level 0's connection limits without changing other clients' policy.
	relay := map[string]json.RawMessage{}
	if raw := levels["0"]; len(raw) > 0 {
		if err := json.Unmarshal(raw, &relay); err != nil {
			return 0, err
		}
	}
	if relay == nil {
		relay = map[string]json.RawMessage{}
	}
	for _, key := range []string{"statsUserUplink", "statsUserDownlink", "statsUserOnline"} {
		relay[key] = json.RawMessage(`false`)
	}
	encoded, err := json.Marshal(relay)
	if err != nil {
		return 0, err
	}
	levels[strconv.FormatUint(uint64(level), 10)] = encoded
	policy["levels"], err = json.Marshal(levels)
	if err != nil {
		return 0, err
	}
	encoded, err = json.Marshal(policy)
	if err != nil {
		return 0, err
	}
	cfg.Policy = encoded
	return level, nil
}

func tuicSocksSettings(emails []string, level uint32) ([]byte, error) {
	raw, err := tuic.SocksInboundSettings(emails, tuic.SocksPassword())
	if err != nil {
		return nil, err
	}
	var settings map[string]json.RawMessage
	if err := json.Unmarshal(raw, &settings); err != nil {
		return nil, err
	}
	settings["userLevel"] = json.RawMessage(strconv.FormatUint(uint64(level), 10))
	return json.Marshal(settings)
}

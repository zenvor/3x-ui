// Package model defines GORM models for the subconverter module.
//
// Models live here (rather than database/model/model.go) so the subconverter
// module stays additive to upstream 3X-UI: pulling new upstream changes never
// touches subconverter files, and our schema evolves on its own cadence.
package model

import "time"

// Subscription maps a public token to a set of 3X-UI inbounds and is the root
// of a Mihomo subscription link.
type Subscription struct {
	Id        int       `json:"id" gorm:"primaryKey;autoIncrement"`
	Token     string    `json:"token" gorm:"uniqueIndex;not null;size:64"`
	Remark    string    `json:"remark" gorm:"not null"`
	MaxIps    int       `json:"limitIp" gorm:"default:1"`   // 0 = unlimited; JSON name matches 3X-UI's Client.LimitIP
	Enabled   bool      `json:"enable" gorm:"default:true"` // JSON name matches 3X-UI's Inbound.Enable
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`

	// Inbounds is the preloadable junction list. omitempty keeps the JSON
	// output compact when a caller does not Preload the relation.
	Inbounds []SubscriptionInbound `json:"inbounds,omitempty" gorm:"foreignKey:SubscriptionId;references:Id"`
}

// SubscriptionInbound is the join row between a subscription and a 3X-UI inbound.
//
// InboundId is a logical reference to model.Inbound in the 3X-UI database; no FK
// constraint is enforced because the two tables live in separate SQLite files.
// ClientEmail empty means "all clients of this inbound"; non-empty means only
// that one client of the inbound is included in the subscription output.
type SubscriptionInbound struct {
	Id             int       `json:"id" gorm:"primaryKey;autoIncrement"`
	SubscriptionId int       `json:"subscriptionId" gorm:"index;not null"`
	InboundId      int       `json:"inboundId" gorm:"not null"`
	ClientEmail    string    `json:"clientEmail"`
	SortOrder      int       `json:"sortOrder" gorm:"default:0"`
	CreatedAt      time.Time `json:"createdAt"`
}

// IpBinding records that a particular IP has consumed one slot of a
// subscription's MaxIps quota. The (subscription_id, ip) pair is unique.
type IpBinding struct {
	Id             int       `json:"id" gorm:"primaryKey;autoIncrement"`
	SubscriptionId int       `json:"subscriptionId" gorm:"not null;uniqueIndex:idx_sub_ip,priority:1"`
	Ip             string    `json:"ip" gorm:"not null;uniqueIndex:idx_sub_ip,priority:2;size:64"`
	BoundAt        time.Time `json:"boundAt"`
	LastSeenAt     time.Time `json:"lastSeenAt"`
}

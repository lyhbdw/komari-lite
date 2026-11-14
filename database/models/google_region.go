package models

import "time"

const (
	GoogleRegionStatusOK      = "ok"
	GoogleRegionStatusUnknown = "unknown"
)

type GoogleRegionProbe struct {
	Country    string     `json:"country,omitempty"`
	Status     string     `json:"status"`
	Reason     string     `json:"reason,omitempty"`
	CheckedAt  *time.Time `json:"checked_at,omitempty"`
	DetectedAt *time.Time `json:"detected_at,omitempty"`
}

type GoogleRegionFamilyChecks struct {
	GoogleAccounts GoogleRegionProbe `json:"google_accounts"`
	YouTube        GoogleRegionProbe `json:"youtube"`
	YouTubePremium GoogleRegionProbe `json:"youtube_premium"`
}

type GoogleRegionChecks struct {
	IPv4      GoogleRegionFamilyChecks `json:"ipv4"`
	IPv6      GoogleRegionFamilyChecks `json:"ipv6"`
	CheckedAt *time.Time               `json:"checked_at,omitempty"`
}

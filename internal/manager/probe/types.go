package probe

// AgentResult is the JSON the embedded probe binary POSTs back to the
// fingerprint probe server.
type AgentResult struct {
	Hostname     string `json:"hostname"`
	OS           string `json:"os"`
	Arch         string `json:"arch"`
	LocalHex     string `json:"local_hex"`
	CompositeHex string `json:"composite_hex"`
	CPUBrand     string `json:"cpu_brand,omitempty"`
	SentAt       string `json:"sent_at"`
}

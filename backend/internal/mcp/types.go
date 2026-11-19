package mcp

type Status struct {
	TenantID            string   `json:"tenant_id"`
	TenantName          string   `json:"tenant_name"`
	Service            string   `json:"service"`
	ProtocolVersion    string   `json:"protocol_version"`
	Authenticated      bool     `json:"authenticated"`
	AdvertiserID       string   `json:"advertiser_id"`
	AwemeID            string   `json:"aweme_id"`
	LiveRoomID         string   `json:"live_room_id"`
	ReadEnabled        bool     `json:"read_enabled"`
	WriteEnabled       bool     `json:"write_enabled"`
	AuthorizationURL   string   `json:"authorization_url,omitempty"`
	Capabilities       []string `json:"capabilities"`
	WriteCapabilities  []string `json:"write_capabilities"`
	OperationMode      string   `json:"operation_mode"`
	RestartRequired    bool     `json:"restart_required"`
	TokenEncrypted     bool     `json:"token_encrypted"`
	TokenStorePresent  bool     `json:"token_store_present"`
	WriteEnabledReason string   `json:"write_enabled_reason"`
}

type MaterialMetrics struct {
	MaterialID       string   `json:"material_id"`
	Title            string   `json:"title"`
	VideoID          string   `json:"video_id,omitempty"`
	AwemeItemID      string   `json:"aweme_item_id,omitempty"`
	Status           string   `json:"status,omitempty"`
	Cost             float64  `json:"cost"`
	ShowCount        int64    `json:"show_count"`
	ClickCount       int64    `json:"click_count"`
	CTR              float64  `json:"ctr"`
	ConvertCount     int64    `json:"convert_count"`
	ConversionRate   float64  `json:"conversion_rate"`
	PayOrderCount    float64  `json:"pay_order_count"`
	PayOrderGMV      float64  `json:"pay_order_gmv"`
	ROI              float64  `json:"roi"`
	GPM              float64  `json:"gpm"`
	CostRank         int      `json:"cost_rank"`
	ProtectedTop3    bool     `json:"protected_top3"`
	EligibleReplace  bool     `json:"eligible_for_replacement"`
	MetricSource     string   `json:"metric_source"`
	MissingMetrics   []string `json:"missing_metrics,omitempty"`
	CTRPassed        *bool    `json:"ctr_passed,omitempty"`
	ConversionPassed *bool    `json:"conversion_passed,omitempty"`
}

type RuleConfig struct {
	MinimumCTR             float64 `json:"minimum_ctr"`
	MinimumConversionRate float64 `json:"minimum_conversion_rate"`
	MinimumMinuteCost     float64 `json:"minimum_minute_cost"`
	ProtectedTopN         int     `json:"protected_top_n"`
}

func DefaultRuleConfig() RuleConfig {
	return RuleConfig{
		MinimumCTR:             0.07,
		MinimumConversionRate: 0.08,
		MinimumMinuteCost:     20,
		ProtectedTopN:         3,
	}
}

type Evaluation struct {
	AdvertiserID      string          `json:"advertiser_id"`
	AdID              string          `json:"ad_id"`
	WindowMinutes     int             `json:"window_minutes"`
	Cost              float64         `json:"cost"`
	MinuteCost        float64         `json:"minute_cost"`
	MinuteCostPassed  bool            `json:"minute_cost_passed"`
	CTR               float64         `json:"ctr"`
	CTRPassed         bool            `json:"ctr_passed"`
	ConversionRate    float64         `json:"conversion_rate"`
	ConversionPassed  bool            `json:"conversion_passed"`
	GPM               float64         `json:"gpm"`
	Status            string          `json:"status"`
	Action            string          `json:"recommended_action"`
	ProtectedTop3     []MaterialMetrics `json:"protected_top3"`
	ReplacementCandidates []MaterialMetrics `json:"replacement_candidates"`
	RuleConfig        RuleConfig      `json:"rule_config"`
	MetricsComplete   bool            `json:"metrics_complete"`
	Warnings          []string        `json:"warnings,omitempty"`
}

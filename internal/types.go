package internal

// ProwJob represents the result for a Prow job run.
type ProwJob struct {
	Name      string `json:"name"`
	URL       string `json:"url"`
	ResultURL string `json:"result_file"`
	Result    string `json:"result"`
}

type Cell struct {
	URL    string
	Result string
}

type Entry struct {
	Variant             string
	InstallSuccess      Cell
	OverallTest         bool
	UpgradeFromCurrent  Cell
	UpgradeFromPrevious Cell
	Serial              Cell
	Parallel            Cell
	CSI                 Cell
}

type Variant struct {
	Name                string
	UpgradeFromCurrent  bool
	UpgradeFromPrevious bool
	Serial              bool
	Parallel            bool
	CSI                 bool
}

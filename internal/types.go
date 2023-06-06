package internal

// ProwJob represents the result for a Prow job run.
type ProwJob struct {
	Name             string `json:"name"`
	URL              string `json:"url"`
	InstallStatusURL string `json:"install_status_file"`
	InstallStatus    string `json:"install_status"`
	ResultURL        string `json:"result_file"`
	Result           string `json:"result"`
}

// Cell holds the information of a "td" in an HTML table.
type Cell struct {
	URL    string
	Result string
}

// Entry is an "row" in the table data.
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

// Variant is a set of prow jobs that test similar characteristics of an OCP installation.
type Variant struct {
	Name                string
	UpgradeFromCurrent  bool
	UpgradeFromPrevious bool
	Serial              bool
	Parallel            bool
	CSI                 bool
}

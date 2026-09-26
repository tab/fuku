package model

import "time"

// Severity is the outcome of a single doctor check
type Severity int

// Severity values, ordered from healthy to fatal
const (
	SeverityOK   Severity = iota
	SeverityIdle          // inactive but expected
	SeverityNote
	SeverityWarn
	SeverityFail
)

// String returns the lowercase name of a Severity (used in JSON output)
func (s Severity) String() string {
	switch s {
	case SeverityOK:
		return "ok"
	case SeverityIdle:
		return "idle"
	case SeverityNote:
		return "note"
	case SeverityWarn:
		return "warn"
	case SeverityFail:
		return "fail"
	default:
		return "unknown"
	}
}

// Category names the report section a check belongs to
type Category string

// Categories, one per report section
const (
	CategoryEnvironment   Category = "environment"
	CategoryConfiguration Category = "configuration"
	CategoryServices      Category = "services"
	CategoryTopology      Category = "topology"
	CategoryRuntime       Category = "runtime"
)

// CheckID identifies a check in the text and JSON reports
type CheckID string

// Check identifiers, grouped by category
const (
	CheckSystem  CheckID = "system"
	CheckRuntime CheckID = "runtime"
	CheckInstall CheckID = "install"

	CheckConfigFile     CheckID = "config.file"
	CheckConfigOverride CheckID = "config.override"
	CheckConfigValidate CheckID = "config.validate"
	CheckConfigSettings CheckID = "config.settings"

	CheckServicesDirectories CheckID = "services.directories"
	CheckServicesDotenv      CheckID = "services.dotenv"
	CheckServicesReadiness   CheckID = "services.readiness"

	CheckTopologyTiers   CheckID = "topology.tiers"
	CheckTopologyProfile CheckID = "topology.profile"

	CheckRuntimeInstance CheckID = "runtime.instance"
	CheckRuntimeSockets  CheckID = "runtime.sockets"
	CheckRuntimePorts    CheckID = "runtime.ports"
)

// Detail is a key-value pair shown in the indented detail block of a result
type Detail struct {
	Key   string
	Value string
}

// Result is the outcome of a single check
type Result struct {
	ID          CheckID
	Category    Category
	Severity    Severity
	Summary     string
	Details     []Detail
	Remediation string
	Duration    time.Duration
}

// Section groups results under a category heading
type Section struct {
	Title   string
	Note    string
	Results []Result
}

// Report is the full output of a doctor run
type Report struct {
	SchemaVersion int
	GeneratedAt   time.Time
	Version       string
	Platform      string
	Sections      []Section
}

// Tally counts results by severity across all sections
func (r *Report) Tally() Tally {
	var t Tally

	for _, s := range r.Sections {
		for _, res := range s.Results {
			switch res.Severity {
			case SeverityOK:
				t.OK++
			case SeverityIdle:
				t.Idle++
			case SeverityNote:
				t.Note++
			case SeverityWarn:
				t.Warn++
			case SeverityFail:
				t.Fail++
			}
		}
	}

	return t
}

// Tally counts results by severity
type Tally struct {
	OK   int
	Idle int
	Note int
	Warn int
	Fail int
}

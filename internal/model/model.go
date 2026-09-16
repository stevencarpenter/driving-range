package model

import "time"

type Challenge struct {
	ID                string    `json:"id"`
	Revision          int       `json:"revision"`
	Track             string    `json:"track"`
	Title             string    `json:"title"`
	Objective         string    `json:"objective"`
	Brief             string    `json:"brief"`
	Difficulty        int       `json:"difficulty"`
	Minutes           int       `json:"minutes"`
	Tools             []string  `json:"tools"`
	Concepts          []string  `json:"concepts"`
	Hints             []string  `json:"hints"`
	Explanation       string    `json:"explanation"`
	ReferenceSolution string    `json:"reference_solution"`
	License           string    `json:"license"`
	Author            string    `json:"author"`
	Profile           string    `json:"profile"`
	Entrypoint        string    `json:"entrypoint"`
	Editor            string    `json:"editor"`
	SubmissionFile    string    `json:"submission_file,omitempty"`
	SubmissionArgv    []string  `json:"submission_argv,omitempty"`
	Validator         Validator `json:"validator"`
	Fixtures          []Fixture `json:"fixtures"`
}

type Fixture struct {
	Name           string            `json:"name"`
	Files          map[string]string `json:"files"`
	ExpectedFiles  map[string]string `json:"expected_files,omitempty"`
	ExpectedStdout string            `json:"expected_stdout,omitempty"`
	Setup          string            `json:"setup,omitempty"`
}

type Validator struct {
	Kind            string         `json:"kind"`
	Version         string         `json:"version"`
	OutputPolicy    string         `json:"output_policy"`
	AllowExtraFiles bool           `json:"allow_extra_files,omitempty"`
	Checks          []CommandCheck `json:"checks,omitempty"`
}

type CommandCheck struct {
	Name           string   `json:"name"`
	Argv           []string `json:"argv"`
	ExpectedStdout string   `json:"expected_stdout"`
	ExitCode       int      `json:"exit_code"`
}

type Assignment struct {
	Date       string `json:"date"`
	Track      string `json:"track"`
	ExerciseID string `json:"exercise_id"`
	Revision   int    `json:"revision"`
	Seed       string `json:"seed"`
}

type CheckResult struct {
	Outcome  string   `json:"outcome"`
	Summary  string   `json:"summary"`
	Details  []string `json:"details,omitempty"`
	ExitCode int      `json:"exit_code"`
}

type Attempt struct {
	ID                 string     `json:"id"`
	ExerciseID         string     `json:"exercise_id"`
	Revision           int        `json:"revision"`
	Track              string     `json:"track"`
	AssignmentDate     string     `json:"assignment_date,omitempty"`
	Seed               string     `json:"seed"`
	Profile            string     `json:"profile"`
	EnvironmentID      string     `json:"environment_id"`
	ValidatorVersion   string     `json:"validator_version"`
	Status             string     `json:"status"`
	CreatedAt          time.Time  `json:"created_at"`
	FinishedAt         *time.Time `json:"finished_at,omitempty"`
	HintLevel          int        `json:"hint_level"`
	SolutionRevealed   bool       `json:"solution_revealed"`
	ExternalAssistance bool       `json:"external_assistance"`
	RetryOf            string     `json:"retry_of,omitempty"`
	Workspace          string     `json:"workspace"`
}

type Session struct {
	ID         int64      `json:"id"`
	AttemptID  string     `json:"attempt_id"`
	StartedAt  time.Time  `json:"started_at"`
	EndedAt    *time.Time `json:"ended_at,omitempty"`
	DurationMS *int64     `json:"duration_ms"`
	Outcome    string     `json:"outcome"`
	ExitCode   *int       `json:"exit_code,omitempty"`
}

type SessionResult struct {
	DurationMS    int64  `json:"duration_ms"`
	DurationKnown bool   `json:"duration_known"`
	Interrupted   bool   `json:"interrupted"`
	ExitCode      int    `json:"exit_code"`
	Error         string `json:"error,omitempty"`
}

type CheckEvent struct {
	ID        int64       `json:"id"`
	AttemptID string      `json:"attempt_id"`
	CreatedAt time.Time   `json:"created_at"`
	Result    CheckResult `json:"result"`
}

type Progress struct {
	Attempt         Attempt `json:"attempt"`
	DurationMS      int64   `json:"duration_ms"`
	UnknownDuration bool    `json:"unknown_duration"`
	Checks          int     `json:"checks"`
	FailedChecks    int     `json:"failed_checks"`
}

type Export struct {
	SchemaVersion int          `json:"schema_version"`
	ExportedAt    time.Time    `json:"exported_at"`
	Attempts      []Attempt    `json:"attempts"`
	Sessions      []Session    `json:"sessions"`
	Checks        []CheckEvent `json:"checks"`
}

func (a Attempt) MatchesAssignment(daily Assignment) bool {
	return a.ExerciseID == daily.ExerciseID && a.AssignmentDate == daily.Date && a.Revision == daily.Revision && a.Seed == daily.Seed && a.Track == daily.Track && a.Profile == "standard"
}

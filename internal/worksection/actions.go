package worksection

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

const ContractVersion = "2026-06-16.2"

type ParamType string

const (
	ParamString ParamType = "string"
	ParamDate   ParamType = "date"
	ParamBool   ParamType = "boolean"
	ParamCSV    ParamType = "csv"
)

// ParamSpec describes one Worksection API parameter.
type ParamSpec struct {
	Name     string    `json:"name"`
	Type     ParamType `json:"type"`
	Required bool      `json:"required,omitempty"`
	Enum     []string  `json:"enum,omitempty"`
	// Pattern, when set, is a regular expression the value must match. It is
	// validated client-side so callers get a clear error instead of a
	// misleading Worksection rejection (e.g. period must be <N>m|h|d). The
	// pattern checks format only; the server still enforces value ranges.
	Pattern     string `json:"pattern,omitempty"`
	Description string `json:"description,omitempty"`
}

// FieldSpec is an advisory response field contract. It is intentionally not a
// full JSON Schema because Worksection fields vary by permissions and extras.
type FieldSpec struct {
	Name        string `json:"name"`
	Type        string `json:"type,omitempty"`
	Description string `json:"description,omitempty"`
}

// ResponseContract is the static, agent-readable response contract for an
// action. It helps choose --fields/--jq before an API call.
type ResponseContract struct {
	ContractVersion   string                 `json:"contract_version"`
	Shape             string                 `json:"response_shape"`
	DataPath          string                 `json:"data_path"`
	CountPath         string                 `json:"count_path"`
	AggregatePath     string                 `json:"aggregate_path,omitempty"`
	ItemShape         []FieldSpec            `json:"item_shape,omitempty"`
	ConditionalFields map[string][]FieldSpec `json:"conditional_fields,omitempty"`
	Notes             []string               `json:"notes,omitempty"`
}

// Action is the complete local contract for a known Worksection action.
type Action struct {
	Name               string           `json:"name"`
	ReadOnly           bool             `json:"read_only"`
	FirstClass         bool             `json:"first_class"`
	Description        string           `json:"description"`
	Required           []string         `json:"required,omitempty"`
	Optional           []string         `json:"optional,omitempty"`
	Params             []ParamSpec      `json:"params,omitempty"`
	AnyOf              [][]string       `json:"any_of,omitempty"`
	ExactlyOneOf       []string         `json:"exactly_one_of,omitempty"`
	AuthModes          []string         `json:"auth_modes,omitempty"`
	OAuthScopes        []string         `json:"oauth_scopes,omitempty"`
	Response           ResponseContract `json:"response"`
	TableColumns       []string         `json:"table_columns,omitempty"`
	Truncation         string           `json:"truncation,omitempty"`
	CompatibilityNotes []string         `json:"compatibility_notes,omitempty"`
	CommandPaths       []string         `json:"command_paths,omitempty"`
}

var readActions = map[string]Action{
	"me": action("me", "Get authorized user info", []string{"oauth2"}, []string{},
		responseObject("data", fields("id:string", "name:string", "email:string", "first_name:string", "last_name:string", "department:string", "role:string", "title:string", "avatar:string", "url:string", "group:object", "services:array")),
		cols("id", "name", "email", "role"), commands("wsectl me")),

	"get_users": action("get_users", "List account users", []string{"oauth2", "admin_token"}, []string{"users_read"},
		responseArray("data", fields("id:string", "name:string", "email:string", "first_name:string", "last_name:string", "department:string", "role:string", "title:string", "group:object")),
		cols("id", "name", "email", "role"), commands("wsectl users list")),

	"get_user_groups": action("get_user_groups", "List user teams", []string{"oauth2", "admin_token"}, []string{"users_read"},
		responseArray("data", fields("id:string", "name:string", "users:array")), cols("id", "name"), commands("wsectl users groups")),

	"get_contacts": action("get_contacts", "List contacts", []string{"oauth2", "admin_token"}, []string{"contacts_read"},
		responseArray("data", fields("id:string", "name:string", "email:string", "company:string", "phone:string", "group:object")), cols("id", "name", "email", "company"), commands("wsectl users contacts")),

	"get_contact_groups": action("get_contact_groups", "List contact folders", []string{"oauth2", "admin_token"}, []string{"contacts_read"},
		responseArray("data", fields("id:string", "name:string", "contacts:array")), cols("id", "name"), commands("wsectl users contact-groups")),

	"get_users_schedule": action("get_users_schedule", "List users' non-working days", []string{"oauth2", "admin_token"}, []string{"users_read"},
		responseObject("data", scheduleFields()), cols("id", "email", "name", "schedule"), commands("wsectl users schedule"),
		params(param("users", ParamCSV, false, nil, "Comma-separated user IDs or emails"), param("datestart", ParamDate, false, nil, "Start date DD.MM.YYYY"), param("dateend", ParamDate, false, nil, "End date DD.MM.YYYY"))),

	"get_projects": action("get_projects", "List projects", []string{"oauth2", "admin_token"}, []string{"projects_read"},
		responseArray("data", fields("id:string", "name:string", "status:string", "date_added:string", "date_start:string", "date_end:string", "user:object", "group:object"),
			conditional("extra=text", fields("text:string")),
			conditional("extra=options", fields("options:object")),
			conditional("extra=users", fields("users:array"))),
		cols("id", "name", "status", "date_start", "date_end"), commands("wsectl projects list"),
		params(param("filter", ParamString, false, []string{"active", "pending", "archive"}, "Project filter. CLI --status archived maps to API filter=archive."), param("extra", ParamCSV, false, []string{"text", "options", "users"}, "Comma-separated extras")),
		notes("Official docs use archived in some places, but Postman/live API expects filter=archive.")),

	"get_project": action("get_project", "Get project", []string{"oauth2", "admin_token"}, []string{"projects_read"},
		responseObject("data", fields("id:string", "name:string", "status:string", "date_added:string", "date_start:string", "date_end:string", "text:string", "options:object", "users:array")),
		cols("id", "name", "status"), commands("wsectl projects get", "wsectl projects team"),
		params(param("id_project", ParamString, true, nil, "Project ID"), param("extra", ParamCSV, false, []string{"text", "options", "users"}, "Comma-separated extras"))),

	"get_project_groups": action("get_project_groups", "List project folders", []string{"oauth2", "admin_token"}, []string{"projects_read"},
		responseArray("data", projectGroupFields()), cols("id", "title", "type", "client"), commands("wsectl projects groups")),

	"get_events": action("get_events", "Get project event history", []string{"oauth2", "admin_token"}, []string{"projects_read"},
		responseArray("data", eventFields()), cols("action", "date_added", "object", "user_from"), commands("wsectl projects events"),
		params(withPattern(param("period", ParamString, true, nil, "Relative period: <N>m|h|d — minutes, hours, or days (e.g. 30m, 24h, 7d)."), `^[1-9]\d*[mhd]$`), param("id_project", ParamString, false, nil, "Optional project ID"))),

	"get_all_tasks": taskListAction("get_all_tasks", "List all account tasks", "wsectl tasks all"),
	"get_tasks":     taskListAction("get_tasks", "List project tasks", "wsectl tasks list", param("id_project", ParamString, true, nil, "Project ID")),
	"get_task": action("get_task", "Get task", []string{"oauth2", "admin_token"}, []string{"tasks_read"},
		taskResponseObject(), cols("id", "name", "status", "date_end"), commands("wsectl tasks get", "wsectl tasks subtasks", "wsectl tasks relations", "wsectl tasks subscribers", "wsectl tasks discussion"),
		params(param("id_task", ParamString, true, nil, "Task ID"), param("extra", ParamCSV, false, []string{"text", "files", "comments", "relations", "subtasks", "subscribers"}, "Comma-separated extras"))),

	"search_tasks": action("search_tasks", "Search tasks", []string{"oauth2", "admin_token"}, []string{"tasks_read"},
		responseArray("data", taskFields()), cols("id", "name", "status", "date_end"), commands("wsectl tasks search"),
		params(param("filter", ParamString, false, nil, "Advanced Worksection task filter"), param("id_project", ParamString, false, nil, "Project ID"), param("id_task", ParamString, false, nil, "Task ID"), param("email_user_to", ParamString, false, nil, "Assignee email"), param("email_user_from", ParamString, false, nil, "Author email"), param("status", ParamString, false, []string{"active", "done", "all"}, "Task status"), param("extra", ParamCSV, false, []string{"text", "files", "comments", "relations", "subtasks", "subscribers"}, "Comma-separated extras")),
		anyOf("filter", "id_project", "id_task", "email_user_to", "email_user_from"),
		notes("The CLI translates --query TEXT to filter=name has 'TEXT'. The raw API parameter is filter, not search.",
			"search_tasks needs at least one of filter, id_project, id_task, email_user_to, email_user_from; status and extra are modifiers, not search criteria.",
			"The advanced filter grammar supports fields name and date_added with operators has, <, >, and (e.g. \"date_added > '01.06.2026' and date_added < '12.06.2026'\"), enabling server-side date-range filtering. Unsupported fields such as status, tag, or priority make Worksection reject the filter with a misleading \"Field is required: filter\".")),

	"get_comments": action("get_comments", "List task comments", []string{"oauth2", "admin_token"}, []string{"comments_read"},
		responseArray("data", fields("id:string", "text:string", "date_added:string", "user:object", "files:array")), cols("id", "date_added", "user", "text"), commands("wsectl comments list"),
		params(param("id_task", ParamString, true, nil, "Task ID"), param("extra", ParamCSV, false, []string{"files"}, "Comma-separated extras"))),

	"get_task_tags":          tagAction("get_task_tags", "List task tags", "wsectl tags task list"),
	"get_task_tag_groups":    tagGroupAction("get_task_tag_groups", "List task tag groups", "wsectl tags task groups"),
	"get_project_tags":       tagAction("get_project_tags", "List project tags", "wsectl tags project list"),
	"get_project_tag_groups": tagGroupAction("get_project_tag_groups", "List project tag groups", "wsectl tags project groups"),

	"get_costs":       costAction("get_costs", "List costs", "wsectl costs list", true),
	"get_costs_total": costAction("get_costs_total", "Get costs total", "wsectl costs total", false),

	"get_timers": action("get_timers", "List enabled timers", []string{"oauth2", "admin_token"}, []string{"costs_read"},
		responseArray("data", timerFields(true)), cols("id", "time", "date_started", "user_from", "task"), commands("wsectl timers list")),

	"get_my_timer": action("get_my_timer", "Get current user timer", []string{"oauth2", "admin_token"}, []string{"costs_read"},
		responseObject("data", timerFields(false)), cols("time", "date_started", "task"), commands("wsectl timers mine"),
		notes("Official docs show get_my_timer as an OAuth-only current-user timer response; the no-active-timer shape can vary by account state.")),

	"get_files": action("get_files", "List files", []string{"oauth2", "admin_token"}, []string{"files_read"},
		responseArray("data", fileFields()), cols("id", "name", "type", "size"), commands("wsectl files list", "wsectl files images"),
		params(param("id_project", ParamString, false, nil, "Project ID"), param("id_task", ParamString, false, nil, "Task ID")),
		exactlyOne("id_project", "id_task")),

	"download": action("download", "Download a file", []string{"oauth2", "admin_token"}, []string{"files_read"},
		responseBinary("body", fields("filename:string", "content_type:string", "bytes:binary")), cols(), commands("wsectl files download"),
		params(param("id_file", ParamString, true, nil, "File ID"))),

	"get_webhooks": action("get_webhooks", "List webhooks", []string{"admin_token", "oauth2"}, []string{},
		responseArray("data", webhookFields()), cols("id", "status", "url", "events"), commands("wsectl webhooks list"),
		notes("Webhook access may require an administrative token or an explicit administrative OAuth scope depending on the Worksection app settings.")),
}

var writeActions = map[string]Action{
	"add_user": {}, "add_user_group": {}, "add_contact": {}, "add_contact_group": {},
	"subscribe": {}, "unsubscribe": {}, "update_users_schedule": {},
	"post_project": {}, "update_project": {}, "close_project": {}, "activate_project": {},
	"add_project_members": {}, "delete_project_members": {}, "add_project_group": {}, "add_project_groups": {},
	"post_task": {}, "update_task": {}, "complete_task": {}, "reopen_task": {},
	"post_comment": {}, "add_task_tags": {}, "update_task_tags": {}, "add_task_tag_groups": {},
	"add_project_tags": {}, "update_project_tags": {}, "add_project_tag_groups": {},
	"add_costs": {}, "update_costs": {}, "delete_costs": {},
	"stop_timer": {}, "start_my_timer": {}, "stop_my_timer": {},
	"add_webhook": {}, "delete_webhook": {},
}

// LookupAction returns metadata for a known Worksection action.
func LookupAction(name string) (Action, bool) {
	if a, ok := readActions[name]; ok {
		return a, true
	}
	if _, ok := writeActions[name]; ok {
		return Action{
			Name:        name,
			ReadOnly:    false,
			Description: "Mutation blocked in read-only build",
			AuthModes:   []string{"oauth2", "admin_token"},
			Response:    unknownResponse(),
		}, true
	}
	return Action{Name: name, Response: unknownResponse()}, false
}

// Actions returns all known read-only and blocked mutation actions.
func Actions() []Action {
	out := make([]Action, 0, len(readActions)+len(writeActions))
	for _, a := range readActions {
		out = append(out, a)
	}
	for name := range writeActions {
		a, _ := LookupAction(name)
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// ValidateAction checks a known action and its parameters before a request.
func ValidateAction(action string, params map[string]string, allowUnknown bool) error {
	spec, known := LookupAction(action)
	if err := validateActionAvailability(action, spec, known, allowUnknown); err != nil || !known {
		return err
	}
	if err := validateProvidedParams(action, params, spec, allowUnknown); err != nil {
		return err
	}
	if err := validateRequiredParams(action, params, spec.Params); err != nil {
		return err
	}
	if err := validateAnyOfParams(action, params, spec.AnyOf); err != nil {
		return err
	}
	return validateExactlyOneParams(action, params, spec.ExactlyOneOf)
}

func validateActionAvailability(action string, spec Action, known, allowUnknown bool) error {
	if known && !spec.ReadOnly {
		return UsageError("This action changes Worksection data and is blocked in the read-only build.")
	}
	if known || allowUnknown {
		return nil
	}
	return UsageError("unknown action %q; pass --allow-unknown to call it", action)
}

func validateProvidedParams(action string, params map[string]string, spec Action, allowUnknown bool) error {
	knownParams := paramSpecMap(spec.Params)
	for name, value := range params {
		if err := validateProvidedParam(action, name, value, knownParams, allowUnknown); err != nil {
			return err
		}
	}
	return nil
}

func paramSpecMap(params []ParamSpec) map[string]ParamSpec {
	knownParams := map[string]ParamSpec{}
	for _, p := range params {
		knownParams[p.Name] = p
	}
	return knownParams
}

func validateProvidedParam(action, name, value string, knownParams map[string]ParamSpec, allowUnknown bool) error {
	if value == "" {
		return nil
	}
	p, ok := knownParams[name]
	if !ok {
		if allowUnknown {
			return nil
		}
		return UsageError("parameter %q is not documented for action %s", name, action)
	}
	if len(p.Enum) > 0 && !enumContainsCSV(p.Enum, value, p.Type == ParamCSV) {
		return enumValidationError(action, name, value, p.Enum)
	}
	if p.Pattern != "" {
		matched, err := regexp.MatchString(p.Pattern, value)
		if err != nil || !matched {
			return UsageError("parameter %q for action %s is invalid (%q); expected %s", name, action, value, p.Description)
		}
	}
	return nil
}

func enumValidationError(action, name, value string, enum []string) error {
	if (action == "get_tasks" || action == "get_all_tasks") && name == "filter" && value == "done" {
		return UsageError("completed tasks are not available through %s; use `wsectl tasks search --status done`", action)
	}
	return UsageError("parameter %q for action %s must be one of: %s", name, action, strings.Join(enum, ", "))
}

func validateRequiredParams(action string, params map[string]string, specs []ParamSpec) error {
	for _, p := range specs {
		if p.Required && strings.TrimSpace(params[p.Name]) == "" {
			return UsageError("parameter %q is required for action %s", p.Name, action)
		}
	}
	return nil
}

func validateAnyOfParams(action string, params map[string]string, groups [][]string) error {
	for _, group := range groups {
		if countPresent(params, group) == 0 {
			return UsageError("action %s requires at least one of: %s", action, strings.Join(group, ", "))
		}
	}
	return nil
}

func validateExactlyOneParams(action string, params map[string]string, group []string) error {
	if len(group) == 0 {
		return nil
	}
	if count := countPresent(params, group); count != 1 {
		return UsageError("action %s requires exactly one of: %s", action, strings.Join(group, ", "))
	}
	return nil
}

func action(name, description string, authModes, scopes []string, response ResponseContract, table []string, commandPaths []string, opts ...func(*Action)) Action {
	a := Action{
		Name:         name,
		ReadOnly:     true,
		FirstClass:   len(commandPaths) > 0,
		Description:  description,
		AuthModes:    authModes,
		OAuthScopes:  scopes,
		Response:     response,
		TableColumns: table,
		CommandPaths: commandPaths,
		Truncation:   "Worksection can return up to 10000 records for some endpoints; completeness can be uncertain for large responses.",
	}
	for _, opt := range opts {
		opt(&a)
	}
	a.Required, a.Optional = requiredOptional(a.Params)
	return a
}

func taskListAction(name, description, commandPath string, extraParams ...ParamSpec) Action {
	params := []ParamSpec{
		param("filter", ParamString, false, []string{"active"}, "Only active is supported by list endpoints; completed tasks use search_tasks."),
		param("extra", ParamCSV, false, []string{"text", "files", "comments", "relations", "subtasks", "subscribers"}, "Comma-separated extras"),
	}
	params = append(extraParams, params...)
	return action(name, description, []string{"oauth2", "admin_token"}, []string{"tasks_read"},
		responseArray("data", taskFields(),
			conditional("extra=text", fields("text:string")),
			conditional("extra=files", fields("files:array")),
			conditional("extra=comments", fields("comments:array")),
			conditional("extra=relations", fields("relations:array")),
			conditional("extra=subtasks", fields("subtasks:array")),
			conditional("extra=subscribers", fields("subscribers:array"))),
		cols("id", "name", "status", "date_end"), commands(commandPath), paramsOpt(params...))
}

func taskResponseObject() ResponseContract {
	return responseObject("data", taskFields(),
		conditional("extra=text", fields("text:string")),
		conditional("extra=files", fields("files:array")),
		conditional("extra=comments", fields("comments:array")),
		conditional("extra=relations", fields("relations:array")),
		conditional("extra=subtasks", fields("subtasks:array")),
		conditional("extra=subscribers", fields("subscribers:array")))
}

func tagAction(name, description, commandPath string) Action {
	return action(name, description, []string{"oauth2", "admin_token"}, []string{"tags_read"},
		responseArray("data", tagFields()), cols("id", "title", "group"), commands(commandPath),
		params(param("group", ParamString, false, nil, "Group ID"), param("type", ParamString, false, []string{"status", "label"}, "Tag type"), param("access", ParamString, false, []string{"public", "private"}, "Tag access")))
}

func tagGroupAction(name, description, commandPath string) Action {
	return action(name, description, []string{"oauth2", "admin_token"}, []string{"tags_read"},
		responseArray("data", tagGroupFields()), cols("id", "title", "type", "access"), commands(commandPath),
		params(param("type", ParamString, false, []string{"status", "label"}, "Tag type"), param("access", ParamString, false, []string{"public", "private"}, "Tag access")))
}

func costAction(name, description, commandPath string, list bool) Action {
	var shape ResponseContract
	// Table columns are mode-specific. costs list has entry rows; costs total is
	// an aggregate object, so it gets no curated columns — forcing the entry
	// columns onto a {total, ...} object would project a blank table.
	var tableCols []string
	if list {
		// costs list returns the cost entries as an array at data, with a
		// server-side summary the API places in a sibling "total" object. The
		// entries are the primary payload; the summary is lifted into
		// meta.aggregate via AggregatePath so data stays a plain array.
		shape = responseArray("data", costEntryFields())
		shape.AggregatePath = "total"
		tableCols = cols("id", "comment", "time", "money", "date", "is_timer")
	} else {
		// costs total is a pure aggregate response: data is the {total, ...}
		// bundle, with optional per-project / per-task breakdowns via extras.
		shape = responseObject("data", costTotalFields(),
			conditional("extra=projects", fields("projects:object")),
			conditional("extra=tasks", fields("tasks:object")),
			conditional("extra=tasks_top_level", fields("tasks:object")))
		tableCols = cols()
	}
	return action(name, description, []string{"oauth2", "admin_token"}, []string{"costs_read"},
		shape, tableCols, commands(commandPath),
		params(param("id_project", ParamString, false, nil, "Project ID"), param("id_task", ParamString, false, nil, "Task ID"), param("datestart", ParamDate, false, nil, "Start date DD.MM.YYYY"), param("dateend", ParamDate, false, nil, "End date DD.MM.YYYY"), param("is_timer", ParamBool, false, []string{"true", "false"}, "Filter timer costs"), param("filter", ParamString, false, nil, "Worksection filter"), param("extra", ParamCSV, false, nil, "Comma-separated extras")),
		notes("The public CLI flag is --timer, but the Worksection API parameter is is_timer."))
}

func scheduleFields() []FieldSpec {
	return fields("id:string", "email:string", "name:string", "group:string", "department:string", "schedule:object")
}

func projectGroupFields() []FieldSpec {
	return fields("id:string", "title:string", "type:string", "client:number")
}

func eventFields() []FieldSpec {
	return fields("action:string", "object:object", "date_added:string", "user_from:object", "new:object", "old:object")
}

func tagFields() []FieldSpec {
	return fields("id:string", "title:string", "name:string", "group:object", "color:string")
}

func tagGroupFields() []FieldSpec {
	return fields("id:string", "title:string", "name:string", "type:string", "access:string")
}

func costEntryFields() []FieldSpec {
	return fields("id:string", "comment:string", "time:string", "money:string", "date:string", "is_timer:boolean", "user_from:object", "task:object")
}

func costTotalFields() []FieldSpec {
	return fields("total:object")
}

func timerFields(includeUser bool) []FieldSpec {
	out := fields("time:string", "date_started:string", "task:object")
	if includeUser {
		out = append(fields("id:string"), append(out, fields("user_from:object")...)...)
	}
	return out
}

func webhookFields() []FieldSpec {
	return fields("id:string", "url:string", "events:string", "status:string", "projects:string")
}

func taskFields() []FieldSpec {
	// Conditional fields carry a schema-visible description so agents inspecting
	// item_shape know presence varies by task state, rather than treating every
	// field as guaranteed.
	return describe(fields("id:string", "name:string", "status:string", "project:object", "user_to:object", "user_from:object", "date_added:string", "date_start:string", "date_end:string", "date_closed:string", "page:string", "priority:string"),
		map[string]string{
			"project":     "Omitted when results are already scoped to a project (e.g. tasks list/search by project).",
			"date_start":  "Present only when the task has a start date set.",
			"date_end":    "Present only when the task has a deadline set.",
			"date_closed": "Present only on closed (done) tasks.",
		})
}

// describe sets Description on the named fields, leaving others unchanged.
func describe(fs []FieldSpec, byName map[string]string) []FieldSpec {
	for i := range fs {
		if d, ok := byName[fs[i].Name]; ok {
			fs[i].Description = d
		}
	}
	return fs
}

func fileFields() []FieldSpec {
	return fields("id:string", "name:string", "type:string", "size:number", "url:string", "page:string", "project:object", "task:object", "date_added:string", "user:object")
}

func param(name string, typ ParamType, required bool, enum []string, description string) ParamSpec {
	return ParamSpec{Name: name, Type: typ, Required: required, Enum: enum, Description: description}
}

func params(params ...ParamSpec) func(*Action) {
	return paramsOpt(params...)
}

func paramsOpt(params ...ParamSpec) func(*Action) {
	return func(a *Action) { a.Params = append(a.Params, params...) }
}

func notes(values ...string) func(*Action) {
	return func(a *Action) {
		a.CompatibilityNotes = append(a.CompatibilityNotes, values...)
		a.Response.Notes = append(a.Response.Notes, values...)
	}
}

func exactlyOne(names ...string) func(*Action) {
	return func(a *Action) { a.ExactlyOneOf = append(a.ExactlyOneOf, names...) }
}

func anyOf(names ...string) func(*Action) {
	return func(a *Action) { a.AnyOf = append(a.AnyOf, names) }
}

func withPattern(p ParamSpec, pattern string) ParamSpec {
	// Compile at package-init time so a malformed pattern panics immediately
	// (in tests / at startup) rather than silently failing validation later.
	regexp.MustCompile(pattern)
	p.Pattern = pattern
	return p
}

func commands(paths ...string) []string { return paths }
func cols(values ...string) []string    { return values }

func responseArray(path string, item []FieldSpec, conditionals ...conditionalFields) ResponseContract {
	return response("array", path, path, item, conditionals...)
}

func responseObject(path string, item []FieldSpec, conditionals ...conditionalFields) ResponseContract {
	return response("object", path, path, item, conditionals...)
}

func responseBinary(path string, item []FieldSpec) ResponseContract {
	return response("binary", path, path, item)
}

func unknownResponse() ResponseContract {
	return response("unknown", "data", "data", nil)
}

func response(shape, dataPath, countPath string, item []FieldSpec, conditionals ...conditionalFields) ResponseContract {
	c := ResponseContract{
		ContractVersion: ContractVersion,
		Shape:           shape,
		DataPath:        dataPath,
		CountPath:       countPath,
		ItemShape:       item,
	}
	if len(conditionals) > 0 {
		c.ConditionalFields = map[string][]FieldSpec{}
		for _, conditional := range conditionals {
			c.ConditionalFields[conditional.Key] = conditional.Fields
		}
	}
	return c
}

type conditionalFields struct {
	Key    string
	Fields []FieldSpec
}

func conditional(key string, fields []FieldSpec) conditionalFields {
	return conditionalFields{Key: key, Fields: fields}
}

func fields(values ...string) []FieldSpec {
	out := make([]FieldSpec, 0, len(values))
	for _, value := range values {
		name, typ, _ := strings.Cut(value, ":")
		out = append(out, FieldSpec{Name: name, Type: typ})
	}
	return out
}

func requiredOptional(params []ParamSpec) ([]string, []string) {
	var required, optional []string
	for _, p := range params {
		if p.Required {
			required = append(required, p.Name)
		} else {
			optional = append(optional, p.Name)
		}
	}
	return required, optional
}

func enumContainsCSV(enum []string, value string, csv bool) bool {
	if !csv {
		return enumContains(enum, value)
	}
	for _, part := range strings.Split(value, ",") {
		if trimmed := strings.TrimSpace(part); trimmed != "" && !enumContains(enum, trimmed) {
			return false
		}
	}
	return true
}

func enumContains(enum []string, value string) bool {
	for _, item := range enum {
		if item == value {
			return true
		}
	}
	return false
}

func countPresent(params map[string]string, names []string) int {
	count := 0
	for _, name := range names {
		if strings.TrimSpace(params[name]) != "" {
			count++
		}
	}
	return count
}

func (a Action) KnownFieldNames() []string {
	seen := map[string]bool{}
	var out []string
	for _, f := range a.Response.ItemShape {
		if !seen[f.Name] {
			seen[f.Name] = true
			out = append(out, f.Name)
		}
	}
	for _, fields := range a.Response.ConditionalFields {
		for _, f := range fields {
			if !seen[f.Name] {
				seen[f.Name] = true
				out = append(out, f.Name)
			}
		}
	}
	return out
}

func (a Action) String() string {
	return fmt.Sprintf("%s (%s)", a.Name, a.Description)
}

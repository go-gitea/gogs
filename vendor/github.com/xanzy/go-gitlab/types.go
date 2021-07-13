//
// Copyright 2021, Sander van Harmelen
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package gitlab

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"time"
)

// AccessControlValue represents an access control value within GitLab,
// used for managing access to certain project features.
//
// GitLab API docs: https://docs.gitlab.com/ce/api/projects.html
type AccessControlValue string

// List of available access control values.
//
// GitLab API docs: https://docs.gitlab.com/ce/api/projects.html
const (
	DisabledAccessControl AccessControlValue = "disabled"
	EnabledAccessControl  AccessControlValue = "enabled"
	PrivateAccessControl  AccessControlValue = "private"
	PublicAccessControl   AccessControlValue = "public"
)

// AccessControl is a helper routine that allocates a new AccessControlValue
// to store v and returns a pointer to it.
func AccessControl(v AccessControlValue) *AccessControlValue {
	p := new(AccessControlValue)
	*p = v
	return p
}

// AccessLevelValue represents a permission level within GitLab.
//
// GitLab API docs: https://docs.gitlab.com/ce/permissions/permissions.html
type AccessLevelValue int

// List of available access levels
//
// GitLab API docs: https://docs.gitlab.com/ce/permissions/permissions.html
const (
	NoPermissions            AccessLevelValue = 0
	MinimalAccessPermissions AccessLevelValue = 5
	GuestPermissions         AccessLevelValue = 10
	ReporterPermissions      AccessLevelValue = 20
	DeveloperPermissions     AccessLevelValue = 30
	MaintainerPermissions    AccessLevelValue = 40
	OwnerPermissions         AccessLevelValue = 50

	// These are deprecated and should be removed in a future version
	MasterPermissions AccessLevelValue = 40
	OwnerPermission   AccessLevelValue = 50
)

// AccessLevel is a helper routine that allocates a new AccessLevelValue
// to store v and returns a pointer to it.
func AccessLevel(v AccessLevelValue) *AccessLevelValue {
	p := new(AccessLevelValue)
	*p = v
	return p
}

// BuildStateValue represents a GitLab build state.
type BuildStateValue string

// These constants represent all valid build states.
const (
	Pending  BuildStateValue = "pending"
	Created  BuildStateValue = "created"
	Running  BuildStateValue = "running"
	Success  BuildStateValue = "success"
	Failed   BuildStateValue = "failed"
	Canceled BuildStateValue = "canceled"
	Skipped  BuildStateValue = "skipped"
	Manual   BuildStateValue = "manual"
)

// BuildState is a helper routine that allocates a new BuildStateValue
// to store v and returns a pointer to it.
func BuildState(v BuildStateValue) *BuildStateValue {
	p := new(BuildStateValue)
	*p = v
	return p
}

// DeploymentStatusValue represents a Gitlab deployment status.
type DeploymentStatusValue string

// These constants represent all valid deployment statuses.
const (
	DeploymentStatusCreated  DeploymentStatusValue = "created"
	DeploymentStatusRunning  DeploymentStatusValue = "running"
	DeploymentStatusSuccess  DeploymentStatusValue = "success"
	DeploymentStatusFailed   DeploymentStatusValue = "failed"
	DeploymentStatusCanceled DeploymentStatusValue = "canceled"
)

// DeploymentStatus is a helper routine that allocates a new
// DeploymentStatusValue to store v and returns a pointer to it.
func DeploymentStatus(v DeploymentStatusValue) *DeploymentStatusValue {
	p := new(DeploymentStatusValue)
	*p = v
	return p
}

// EventTypeValue represents actions type for contribution events
type EventTypeValue string

// List of available action type
//
// GitLab API docs: https://docs.gitlab.com/ce/api/events.html#action-types
const (
	CreatedEventType   EventTypeValue = "created"
	UpdatedEventType   EventTypeValue = "updated"
	ClosedEventType    EventTypeValue = "closed"
	ReopenedEventType  EventTypeValue = "reopened"
	PushedEventType    EventTypeValue = "pushed"
	CommentedEventType EventTypeValue = "commented"
	MergedEventType    EventTypeValue = "merged"
	JoinedEventType    EventTypeValue = "joined"
	LeftEventType      EventTypeValue = "left"
	DestroyedEventType EventTypeValue = "destroyed"
	ExpiredEventType   EventTypeValue = "expired"
)

// EventTargetTypeValue represents actions type value for contribution events
type EventTargetTypeValue string

// List of available action type
//
// GitLab API docs: https://docs.gitlab.com/ce/api/events.html#target-types
const (
	IssueEventTargetType        EventTargetTypeValue = "issue"
	MilestoneEventTargetType    EventTargetTypeValue = "milestone"
	MergeRequestEventTargetType EventTargetTypeValue = "merge_request"
	NoteEventTargetType         EventTargetTypeValue = "note"
	ProjectEventTargetType      EventTargetTypeValue = "project"
	SnippetEventTargetType      EventTargetTypeValue = "snippet"
	UserEventTargetType         EventTargetTypeValue = "user"
)

// FileActionValue represents the available actions that can be performed on a file.
//
// GitLab API docs: https://docs.gitlab.com/ce/api/commits.html#create-a-commit-with-multiple-files-and-actions
type FileActionValue string

// The available file actions.
const (
	FileCreate FileActionValue = "create"
	FileDelete FileActionValue = "delete"
	FileMove   FileActionValue = "move"
	FileUpdate FileActionValue = "update"
	FileChmod  FileActionValue = "chmod"
)

// FileAction is a helper routine that allocates a new FileActionValue value
// to store v and returns a pointer to it.
func FileAction(v FileActionValue) *FileActionValue {
	p := new(FileActionValue)
	*p = v
	return p
}

// ISOTime represents an ISO 8601 formatted date
type ISOTime time.Time

// ISO 8601 date format
const iso8601 = "2006-01-02"

// MarshalJSON implements the json.Marshaler interface
func (t ISOTime) MarshalJSON() ([]byte, error) {
	if y := time.Time(t).Year(); y < 0 || y >= 10000 {
		// ISO 8901 uses 4 digits for the years
		return nil, errors.New("json: ISOTime year outside of range [0,9999]")
	}

	b := make([]byte, 0, len(iso8601)+2)
	b = append(b, '"')
	b = time.Time(t).AppendFormat(b, iso8601)
	b = append(b, '"')

	return b, nil
}

// UnmarshalJSON implements the json.Unmarshaler interface
func (t *ISOTime) UnmarshalJSON(data []byte) error {
	// Ignore null, like in the main JSON package
	if string(data) == "null" {
		return nil
	}

	isotime, err := time.Parse(`"`+iso8601+`"`, string(data))
	*t = ISOTime(isotime)

	return err
}

// EncodeValues implements the query.Encoder interface
func (t *ISOTime) EncodeValues(key string, v *url.Values) error {
	if t == nil || (time.Time(*t)).IsZero() {
		return nil
	}
	v.Add(key, t.String())
	return nil
}

// String implements the Stringer interface
func (t ISOTime) String() string {
	return time.Time(t).Format(iso8601)
}

// LinkTypeValue represents a release link type.
type LinkTypeValue string

// List of available release link types
//
// GitLab API docs:
// https://docs.gitlab.com/ee/api/releases/links.html#create-a-link
const (
	ImageLinkType   LinkTypeValue = "image"
	OtherLinkType   LinkTypeValue = "other"
	PackageLinkType LinkTypeValue = "package"
	RunbookLinkType LinkTypeValue = "runbook"
)

// LinkType is a helper routine that allocates a new LinkType value
// to store v and returns a pointer to it.
func LinkType(v LinkTypeValue) *LinkTypeValue {
	p := new(LinkTypeValue)
	*p = v
	return p
}

// LicenseApprovalStatusValue describe the approval statuses of a license.
//
// GitLab API docs: https://docs.gitlab.com/ee/api/managed_licenses.html
type LicenseApprovalStatusValue string

// List of available license approval statuses.
const (
	LicenseApproved    LicenseApprovalStatusValue = "approved"
	LicenseBlacklisted LicenseApprovalStatusValue = "blacklisted"
)

// LicenseApprovalStatus is a helper routine that allocates a new license
// approval status value to store v and returns a pointer to it.
func LicenseApprovalStatus(v LicenseApprovalStatusValue) *LicenseApprovalStatusValue {
	p := new(LicenseApprovalStatusValue)
	*p = v
	return p
}

// MergeMethodValue represents a project merge type within GitLab.
//
// GitLab API docs: https://docs.gitlab.com/ce/api/projects.html#project-merge-method
type MergeMethodValue string

// List of available merge type
//
// GitLab API docs: https://docs.gitlab.com/ce/api/projects.html#project-merge-method
const (
	NoFastForwardMerge MergeMethodValue = "merge"
	FastForwardMerge   MergeMethodValue = "ff"
	RebaseMerge        MergeMethodValue = "rebase_merge"
)

// MergeMethod is a helper routine that allocates a new MergeMethod
// to sotre v and returns a pointer to it.
func MergeMethod(v MergeMethodValue) *MergeMethodValue {
	p := new(MergeMethodValue)
	*p = v
	return p
}

// NoteTypeValue represents the type of a Note.
type NoteTypeValue string

// List of available note types.
const (
	DiffNote       NoteTypeValue = "DiffNote"
	DiscussionNote NoteTypeValue = "DiscussionNote"
	GenericNote    NoteTypeValue = "Note"
	LegacyDiffNote NoteTypeValue = "LegacyDiffNote"
)

// NoteType is a helper routine that allocates a new NoteTypeValue to
// store v and returns a pointer to it.
func NoteType(v NoteTypeValue) *NoteTypeValue {
	p := new(NoteTypeValue)
	*p = v
	return p
}

// NotificationLevelValue represents a notification level.
type NotificationLevelValue int

// String implements the fmt.Stringer interface.
func (l NotificationLevelValue) String() string {
	return notificationLevelNames[l]
}

// MarshalJSON implements the json.Marshaler interface.
func (l NotificationLevelValue) MarshalJSON() ([]byte, error) {
	return json.Marshal(l.String())
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (l *NotificationLevelValue) UnmarshalJSON(data []byte) error {
	var raw interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	switch raw := raw.(type) {
	case float64:
		*l = NotificationLevelValue(raw)
	case string:
		*l = notificationLevelTypes[raw]
	case nil:
		// No action needed.
	default:
		return fmt.Errorf("json: cannot unmarshal %T into Go value of type %T", raw, *l)
	}

	return nil
}

// List of valid notification levels.
const (
	DisabledNotificationLevel NotificationLevelValue = iota
	ParticipatingNotificationLevel
	WatchNotificationLevel
	GlobalNotificationLevel
	MentionNotificationLevel
	CustomNotificationLevel
)

var notificationLevelNames = [...]string{
	"disabled",
	"participating",
	"watch",
	"global",
	"mention",
	"custom",
}

var notificationLevelTypes = map[string]NotificationLevelValue{
	"disabled":      DisabledNotificationLevel,
	"participating": ParticipatingNotificationLevel,
	"watch":         WatchNotificationLevel,
	"global":        GlobalNotificationLevel,
	"mention":       MentionNotificationLevel,
	"custom":        CustomNotificationLevel,
}

// NotificationLevel is a helper routine that allocates a new NotificationLevelValue
// to store v and returns a pointer to it.
func NotificationLevel(v NotificationLevelValue) *NotificationLevelValue {
	p := new(NotificationLevelValue)
	*p = v
	return p
}

// ProjectCreationLevelValue represents a project creation level within GitLab.
//
// GitLab API docs: https://docs.gitlab.com/ce/api/
type ProjectCreationLevelValue string

// List of available project creation levels.
//
// GitLab API docs: https://docs.gitlab.com/ce/api/
const (
	NoOneProjectCreation      ProjectCreationLevelValue = "noone"
	MaintainerProjectCreation ProjectCreationLevelValue = "maintainer"
	DeveloperProjectCreation  ProjectCreationLevelValue = "developer"
)

// ProjectCreationLevel is a helper routine that allocates a new ProjectCreationLevelValue
// to store v and returns a pointer to it.
func ProjectCreationLevel(v ProjectCreationLevelValue) *ProjectCreationLevelValue {
	p := new(ProjectCreationLevelValue)
	*p = v
	return p
}

// SubGroupCreationLevelValue represents a sub group creation level within GitLab.
//
// GitLab API docs: https://docs.gitlab.com/ce/api/
type SubGroupCreationLevelValue string

// List of available sub group creation levels.
//
// GitLab API docs: https://docs.gitlab.com/ce/api/
const (
	OwnerSubGroupCreationLevelValue      SubGroupCreationLevelValue = "owner"
	MaintainerSubGroupCreationLevelValue SubGroupCreationLevelValue = "maintainer"
)

// SubGroupCreationLevel is a helper routine that allocates a new SubGroupCreationLevelValue
// to store v and returns a pointer to it.
func SubGroupCreationLevel(v SubGroupCreationLevelValue) *SubGroupCreationLevelValue {
	p := new(SubGroupCreationLevelValue)
	*p = v
	return p
}

// TasksCompletionStatus represents tasks of the issue/merge request.
type TasksCompletionStatus struct {
	Count          int `json:"count"`
	CompletedCount int `json:"completed_count"`
}

// TodoAction represents the available actions that can be performed on a todo.
//
// GitLab API docs: https://docs.gitlab.com/ce/api/todos.html
type TodoAction string

// The available todo actions.
const (
	TodoAssigned          TodoAction = "assigned"
	TodoMentioned         TodoAction = "mentioned"
	TodoBuildFailed       TodoAction = "build_failed"
	TodoMarked            TodoAction = "marked"
	TodoApprovalRequired  TodoAction = "approval_required"
	TodoDirectlyAddressed TodoAction = "directly_addressed"
)

// TodoTargetType represents the available target that can be linked to a todo.
//
// GitLab API docs: https://docs.gitlab.com/ce/api/todos.html
type TodoTargetType string

const (
	TodoTargetAlertManagement  TodoTargetType = "AlertManagement::Alert"
	TodoTargetDesignManagement TodoTargetType = "DesignManagement::Design"
	TodoTargetIssue            TodoTargetType = "Issue"
	TodoTargetMergeRequest     TodoTargetType = "MergeRequest"
)

// VariableTypeValue represents a variable type within GitLab.
//
// GitLab API docs: https://docs.gitlab.com/ce/api/
type VariableTypeValue string

// List of available variable types.
//
// GitLab API docs: https://docs.gitlab.com/ce/api/
const (
	EnvVariableType  VariableTypeValue = "env_var"
	FileVariableType VariableTypeValue = "file"
)

// VariableType is a helper routine that allocates a new VariableTypeValue
// to store v and returns a pointer to it.
func VariableType(v VariableTypeValue) *VariableTypeValue {
	p := new(VariableTypeValue)
	*p = v
	return p
}

// VisibilityValue represents a visibility level within GitLab.
//
// GitLab API docs: https://docs.gitlab.com/ce/api/
type VisibilityValue string

// List of available visibility levels.
//
// GitLab API docs: https://docs.gitlab.com/ce/api/
const (
	PrivateVisibility  VisibilityValue = "private"
	InternalVisibility VisibilityValue = "internal"
	PublicVisibility   VisibilityValue = "public"
)

// Visibility is a helper routine that allocates a new VisibilityValue
// to store v and returns a pointer to it.
func Visibility(v VisibilityValue) *VisibilityValue {
	p := new(VisibilityValue)
	*p = v
	return p
}

// WikiFormatValue represents the available wiki formats.
//
// GitLab API docs: https://docs.gitlab.com/ce/api/wikis.html
type WikiFormatValue string

// The available wiki formats.
const (
	WikiFormatMarkdown WikiFormatValue = "markdown"
	WikiFormatRDoc     WikiFormatValue = "rdoc"
	WikiFormatASCIIDoc WikiFormatValue = "asciidoc"
	WikiFormatOrg      WikiFormatValue = "org"
)

// WikiFormat is a helper routine that allocates a new WikiFormatValue
// to store v and returns a pointer to it.
func WikiFormat(v WikiFormatValue) *WikiFormatValue {
	p := new(WikiFormatValue)
	*p = v
	return p
}

// Bool is a helper routine that allocates a new bool value
// to store v and returns a pointer to it.
func Bool(v bool) *bool {
	p := new(bool)
	*p = v
	return p
}

// Int is a helper routine that allocates a new int32 value
// to store v and returns a pointer to it, but unlike Int32
// its argument value is an int.
func Int(v int) *int {
	p := new(int)
	*p = v
	return p
}

// String is a helper routine that allocates a new string value
// to store v and returns a pointer to it.
func String(v string) *string {
	p := new(string)
	*p = v
	return p
}

// Time is a helper routine that allocates a new time.Time value
// to store v and returns a pointer to it.
func Time(v time.Time) *time.Time {
	p := new(time.Time)
	*p = v
	return p
}

// BoolValue is a boolean value with advanced json unmarshaling features.
type BoolValue bool

// UnmarshalJSON allows 1, 0, "true", and "false" to be considered as boolean values
// Needed for:
// https://gitlab.com/gitlab-org/gitlab-ce/issues/50122
// https://gitlab.com/gitlab-org/gitlab/-/issues/233941
// https://github.com/gitlabhq/terraform-provider-gitlab/issues/348
func (t *BoolValue) UnmarshalJSON(b []byte) error {
	switch string(b) {
	case `"1"`:
		*t = true
		return nil
	case `"0"`:
		*t = false
		return nil
	case `"true"`:
		*t = true
		return nil
	case `"false"`:
		*t = false
		return nil
	default:
		var v bool
		err := json.Unmarshal(b, &v)
		*t = BoolValue(v)
		return err
	}
}

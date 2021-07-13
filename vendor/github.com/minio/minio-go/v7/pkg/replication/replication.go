/*
 * MinIO Client (C) 2020 MinIO, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package replication

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/rs/xid"
)

var errInvalidFilter = fmt.Errorf("invalid filter")

// OptionType specifies operation to be performed on config
type OptionType string

const (
	// AddOption specifies addition of rule to config
	AddOption OptionType = "Add"
	// SetOption specifies modification of existing rule to config
	SetOption OptionType = "Set"

	// RemoveOption specifies rule options are for removing a rule
	RemoveOption OptionType = "Remove"
	// ImportOption is for getting current config
	ImportOption OptionType = "Import"
)

// Options represents options to set a replication configuration rule
type Options struct {
	Op                      OptionType
	ID                      string
	Prefix                  string
	RuleStatus              string
	Priority                string
	TagString               string
	StorageClass            string
	RoleArn                 string
	DestBucket              string
	IsTagSet                bool
	IsSCSet                 bool
	ReplicateDeletes        string // replicate versioned deletes
	ReplicateDeleteMarkers  string // replicate soft deletes
	ReplicaSync             string // replicate replica metadata modifications
	ExistingObjectReplicate string
}

// Tags returns a slice of tags for a rule
func (opts Options) Tags() ([]Tag, error) {
	var tagList []Tag
	tagTokens := strings.Split(opts.TagString, "&")
	for _, tok := range tagTokens {
		if tok == "" {
			break
		}
		kv := strings.SplitN(tok, "=", 2)
		if len(kv) != 2 {
			return []Tag{}, fmt.Errorf("tags should be entered as comma separated k=v pairs")
		}
		tagList = append(tagList, Tag{
			Key:   kv[0],
			Value: kv[1],
		})
	}
	return tagList, nil
}

// Config - replication configuration specified in
// https://docs.aws.amazon.com/AmazonS3/latest/dev/replication-add-config.html
type Config struct {
	XMLName xml.Name `xml:"ReplicationConfiguration" json:"-"`
	Rules   []Rule   `xml:"Rule" json:"Rules"`
	Role    string   `xml:"Role" json:"Role"`
}

// Empty returns true if config is not set
func (c *Config) Empty() bool {
	return len(c.Rules) == 0
}

// AddRule adds a new rule to existing replication config. If a rule exists with the
// same ID, then the rule is replaced.
func (c *Config) AddRule(opts Options) error {
	priority, err := strconv.Atoi(opts.Priority)
	if err != nil {
		return err
	}
	if opts.RoleArn != c.Role && c.Role != "" {
		return fmt.Errorf("role ARN does not match existing configuration")
	}
	var status Status
	// toggle rule status for edit option
	switch opts.RuleStatus {
	case "enable":
		status = Enabled
	case "disable":
		status = Disabled
	default:
		return fmt.Errorf("rule state should be either [enable|disable]")
	}

	tags, err := opts.Tags()
	if err != nil {
		return err
	}
	andVal := And{
		Tags: tags,
	}
	filter := Filter{Prefix: opts.Prefix}
	// only a single tag is set.
	if opts.Prefix == "" && len(tags) == 1 {
		filter.Tag = tags[0]
	}
	// both prefix and tag are present
	if len(andVal.Tags) > 1 || opts.Prefix != "" {
		filter.And = andVal
		filter.And.Prefix = opts.Prefix
		filter.Prefix = ""
		filter.Tag = Tag{}
	}
	if opts.ID == "" {
		opts.ID = xid.New().String()
	}
	arnStr := opts.RoleArn
	if opts.RoleArn == "" {
		arnStr = c.Role
	}
	if arnStr == "" {
		return fmt.Errorf("role ARN required")
	}
	tokens := strings.Split(arnStr, ":")
	if len(tokens) != 6 {
		return fmt.Errorf("invalid format for replication Arn")
	}
	if c.Role == "" {
		c.Role = arnStr
	}
	destBucket := opts.DestBucket
	// ref https://docs.aws.amazon.com/AmazonS3/latest/dev/s3-arn-format.html
	if btokens := strings.Split(destBucket, ":"); len(btokens) != 6 {
		if len(btokens) == 1 {
			destBucket = fmt.Sprintf("arn:aws:s3:::%s", destBucket)
		} else {
			return fmt.Errorf("destination bucket needs to be in Arn format")
		}
	}
	dmStatus := Disabled
	if opts.ReplicateDeleteMarkers != "" {
		switch opts.ReplicateDeleteMarkers {
		case "enable":
			dmStatus = Enabled
		case "disable":
			dmStatus = Disabled
		default:
			return fmt.Errorf("ReplicateDeleteMarkers should be either enable|disable")
		}
	}

	vDeleteStatus := Disabled
	if opts.ReplicateDeletes != "" {
		switch opts.ReplicateDeletes {
		case "enable":
			vDeleteStatus = Enabled
		case "disable":
			vDeleteStatus = Disabled
		default:
			return fmt.Errorf("ReplicateDeletes should be either enable|disable")
		}
	}
	var replicaSync Status
	// replica sync is by default Enabled, unless specified.
	switch opts.ReplicaSync {
	case "enable", "":
		replicaSync = Enabled
	case "disable":
		replicaSync = Disabled
	default:
		return fmt.Errorf("replica metadata sync should be either [enable|disable]")
	}

	var existingStatus Status
	if opts.ExistingObjectReplicate != "" {
		switch opts.ExistingObjectReplicate {
		case "enable":
			existingStatus = Enabled
		case "disable", "":
			existingStatus = Disabled
		default:
			return fmt.Errorf("existingObjectReplicate should be either enable|disable")
		}
	}
	newRule := Rule{
		ID:       opts.ID,
		Priority: priority,
		Status:   status,
		Filter:   filter,
		Destination: Destination{
			Bucket:       destBucket,
			StorageClass: opts.StorageClass,
		},
		DeleteMarkerReplication: DeleteMarkerReplication{Status: dmStatus},
		DeleteReplication:       DeleteReplication{Status: vDeleteStatus},
		// MinIO enables replica metadata syncing by default in the case of bi-directional replication to allow
		// automatic failover as the expectation in this case is that replica and source should be identical.
		// However AWS leaves this configurable https://docs.aws.amazon.com/AmazonS3/latest/dev/replication-for-metadata-changes.html
		SourceSelectionCriteria: SourceSelectionCriteria{
			ReplicaModifications: ReplicaModifications{
				Status: replicaSync,
			},
		},
		// By default disable existing object replication unless selected
		ExistingObjectReplication: ExistingObjectReplication{
			Status: existingStatus,
		},
	}

	// validate rule after overlaying priority for pre-existing rule being disabled.
	if err := newRule.Validate(); err != nil {
		return err
	}
	for _, rule := range c.Rules {
		if rule.Priority == newRule.Priority {
			return fmt.Errorf("priority must be unique. Replication configuration already has a rule with this priority")
		}
		if rule.Destination.Bucket != newRule.Destination.Bucket {
			return fmt.Errorf("the destination bucket must be same for all rules")
		}
		if rule.ID == newRule.ID {
			return fmt.Errorf("a rule exists with this ID")
		}
	}

	c.Rules = append(c.Rules, newRule)
	return nil
}

// EditRule modifies an existing rule in replication config
func (c *Config) EditRule(opts Options) error {
	if opts.ID == "" {
		return fmt.Errorf("rule ID missing")
	}
	rIdx := -1
	var newRule Rule
	for i, rule := range c.Rules {
		if rule.ID == opts.ID {
			rIdx = i
			newRule = rule
			break
		}
	}
	if rIdx < 0 {
		return fmt.Errorf("rule with ID %s not found in replication configuration", opts.ID)
	}
	prefixChg := opts.Prefix != newRule.Prefix()
	if opts.IsTagSet || prefixChg {
		prefix := newRule.Prefix()
		if prefix != opts.Prefix {
			prefix = opts.Prefix
		}
		tags := []Tag{newRule.Filter.Tag}
		if len(newRule.Filter.And.Tags) != 0 {
			tags = newRule.Filter.And.Tags
		}
		var err error
		if opts.IsTagSet {
			tags, err = opts.Tags()
			if err != nil {
				return err
			}
		}
		andVal := And{
			Tags: tags,
		}

		filter := Filter{Prefix: prefix}
		// only a single tag is set.
		if prefix == "" && len(tags) == 1 {
			filter.Tag = tags[0]
		}
		// both prefix and tag are present
		if len(andVal.Tags) > 1 || prefix != "" {
			filter.And = andVal
			filter.And.Prefix = prefix
			filter.Prefix = ""
			filter.Tag = Tag{}
		}
		newRule.Filter = filter
	}

	// toggle rule status for edit option
	if opts.RuleStatus != "" {
		switch opts.RuleStatus {
		case "enable":
			newRule.Status = Enabled
		case "disable":
			newRule.Status = Disabled
		default:
			return fmt.Errorf("rule state should be either [enable|disable]")
		}
	}
	// set DeleteMarkerReplication rule status for edit option
	if opts.ReplicateDeleteMarkers != "" {
		switch opts.ReplicateDeleteMarkers {
		case "enable":
			newRule.DeleteMarkerReplication.Status = Enabled
		case "disable":
			newRule.DeleteMarkerReplication.Status = Disabled
		default:
			return fmt.Errorf("ReplicateDeleteMarkers state should be either [enable|disable]")
		}
	}

	// set DeleteReplication rule status for edit option. This is a MinIO specific
	// option to replicate versioned deletes
	if opts.ReplicateDeletes != "" {
		switch opts.ReplicateDeletes {
		case "enable":
			newRule.DeleteReplication.Status = Enabled
		case "disable":
			newRule.DeleteReplication.Status = Disabled
		default:
			return fmt.Errorf("ReplicateDeletes state should be either [enable|disable]")
		}
	}

	if opts.ReplicaSync != "" {
		switch opts.ReplicaSync {
		case "enable", "":
			newRule.SourceSelectionCriteria.ReplicaModifications.Status = Enabled
		case "disable":
			newRule.SourceSelectionCriteria.ReplicaModifications.Status = Disabled
		default:
			return fmt.Errorf("replica metadata sync should be either [enable|disable]")
		}
	}
	fmt.Println("opts.ExistingObjectReplicate>", opts.ExistingObjectReplicate)
	if opts.ExistingObjectReplicate != "" {
		switch opts.ExistingObjectReplicate {
		case "enable":
			newRule.ExistingObjectReplication.Status = Enabled
		case "disable":
			newRule.ExistingObjectReplication.Status = Disabled
		default:
			return fmt.Errorf("existingObjectsReplication state should be either [enable|disable]")
		}
	}
	if opts.IsSCSet {
		newRule.Destination.StorageClass = opts.StorageClass
	}
	if opts.Priority != "" {
		priority, err := strconv.Atoi(opts.Priority)
		if err != nil {
			return err
		}
		newRule.Priority = priority
	}
	if opts.DestBucket != "" {
		destBucket := opts.DestBucket
		// ref https://docs.aws.amazon.com/AmazonS3/latest/dev/s3-arn-format.html
		if btokens := strings.Split(opts.DestBucket, ":"); len(btokens) != 6 {
			if len(btokens) == 1 {
				destBucket = fmt.Sprintf("arn:aws:s3:::%s", destBucket)
			} else {
				return fmt.Errorf("destination bucket needs to be in Arn format")
			}
		}
		newRule.Destination.Bucket = destBucket
	}
	// validate rule
	if err := newRule.Validate(); err != nil {
		return err
	}
	// ensure priority and destination bucket restrictions are not violated
	for idx, rule := range c.Rules {
		if rule.Priority == newRule.Priority && rIdx != idx {
			return fmt.Errorf("priority must be unique. Replication configuration already has a rule with this priority")
		}
		if rule.Destination.Bucket != newRule.Destination.Bucket {
			return fmt.Errorf("the destination bucket must be same for all rules")
		}
	}

	c.Rules[rIdx] = newRule
	return nil
}

// RemoveRule removes a rule from replication config.
func (c *Config) RemoveRule(opts Options) error {
	var newRules []Rule
	ruleFound := false
	for _, rule := range c.Rules {
		if rule.ID != opts.ID {
			newRules = append(newRules, rule)
			continue
		}
		ruleFound = true
	}
	if !ruleFound {
		return fmt.Errorf("Rule with ID %s not found", opts.ID)
	}
	if len(newRules) == 0 {
		return fmt.Errorf("replication configuration should have at least one rule")
	}
	c.Rules = newRules
	return nil

}

// Rule - a rule for replication configuration.
type Rule struct {
	XMLName                   xml.Name                  `xml:"Rule" json:"-"`
	ID                        string                    `xml:"ID,omitempty"`
	Status                    Status                    `xml:"Status"`
	Priority                  int                       `xml:"Priority"`
	DeleteMarkerReplication   DeleteMarkerReplication   `xml:"DeleteMarkerReplication"`
	DeleteReplication         DeleteReplication         `xml:"DeleteReplication"`
	Destination               Destination               `xml:"Destination"`
	Filter                    Filter                    `xml:"Filter" json:"Filter"`
	SourceSelectionCriteria   SourceSelectionCriteria   `xml:"SourceSelectionCriteria" json:"SourceSelectionCriteria"`
	ExistingObjectReplication ExistingObjectReplication `xml:"ExistingObjectReplication,omitempty" json:"ExistingObjectReplication,omitempty"`
}

// Validate validates the rule for correctness
func (r Rule) Validate() error {
	if err := r.validateID(); err != nil {
		return err
	}
	if err := r.validateStatus(); err != nil {
		return err
	}
	if err := r.validateFilter(); err != nil {
		return err
	}

	if r.Priority < 0 && r.Status == Enabled {
		return fmt.Errorf("priority must be set for the rule")
	}

	if err := r.validateStatus(); err != nil {
		return err
	}
	return r.ExistingObjectReplication.Validate()
}

// validateID - checks if ID is valid or not.
func (r Rule) validateID() error {
	// cannot be longer than 255 characters
	if len(r.ID) > 255 {
		return fmt.Errorf("ID must be less than 255 characters")
	}
	return nil
}

// validateStatus - checks if status is valid or not.
func (r Rule) validateStatus() error {
	// Status can't be empty
	if len(r.Status) == 0 {
		return fmt.Errorf("status cannot be empty")
	}

	// Status must be one of Enabled or Disabled
	if r.Status != Enabled && r.Status != Disabled {
		return fmt.Errorf("status must be set to either Enabled or Disabled")
	}
	return nil
}

func (r Rule) validateFilter() error {
	if err := r.Filter.Validate(); err != nil {
		return err
	}
	return nil
}

// Prefix - a rule can either have prefix under <filter></filter> or under
// <filter><and></and></filter>. This method returns the prefix from the
// location where it is available
func (r Rule) Prefix() string {
	if r.Filter.Prefix != "" {
		return r.Filter.Prefix
	}
	return r.Filter.And.Prefix
}

// Tags - a rule can either have tag under <filter></filter> or under
// <filter><and></and></filter>. This method returns all the tags from the
// rule in the format tag1=value1&tag2=value2
func (r Rule) Tags() string {
	ts := []Tag{r.Filter.Tag}
	if len(r.Filter.And.Tags) != 0 {
		ts = r.Filter.And.Tags
	}

	var buf bytes.Buffer
	for _, t := range ts {
		if buf.Len() > 0 {
			buf.WriteString("&")
		}
		buf.WriteString(t.String())
	}
	return buf.String()
}

// Filter - a filter for a replication configuration Rule.
type Filter struct {
	XMLName xml.Name `xml:"Filter" json:"-"`
	Prefix  string   `json:"Prefix,omitempty"`
	And     And      `xml:"And,omitempty" json:"And,omitempty"`
	Tag     Tag      `xml:"Tag,omitempty" json:"Tag,omitempty"`
}

// Validate - validates the filter element
func (f Filter) Validate() error {
	// A Filter must have exactly one of Prefix, Tag, or And specified.
	if !f.And.isEmpty() {
		if f.Prefix != "" {
			return errInvalidFilter
		}
		if !f.Tag.IsEmpty() {
			return errInvalidFilter
		}
	}
	if f.Prefix != "" {
		if !f.Tag.IsEmpty() {
			return errInvalidFilter
		}
	}
	if !f.Tag.IsEmpty() {
		if err := f.Tag.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// Tag - a tag for a replication configuration Rule filter.
type Tag struct {
	XMLName xml.Name `json:"-"`
	Key     string   `xml:"Key,omitempty" json:"Key,omitempty"`
	Value   string   `xml:"Value,omitempty" json:"Value,omitempty"`
}

func (tag Tag) String() string {
	if tag.IsEmpty() {
		return ""
	}
	return tag.Key + "=" + tag.Value
}

// IsEmpty returns whether this tag is empty or not.
func (tag Tag) IsEmpty() bool {
	return tag.Key == ""
}

// Validate checks this tag.
func (tag Tag) Validate() error {
	if len(tag.Key) == 0 || utf8.RuneCountInString(tag.Key) > 128 {
		return fmt.Errorf("invalid Tag Key")
	}

	if utf8.RuneCountInString(tag.Value) > 256 {
		return fmt.Errorf("invalid Tag Value")
	}
	return nil
}

// Destination - destination in ReplicationConfiguration.
type Destination struct {
	XMLName      xml.Name `xml:"Destination" json:"-"`
	Bucket       string   `xml:"Bucket" json:"Bucket"`
	StorageClass string   `xml:"StorageClass,omitempty" json:"StorageClass,omitempty"`
}

// And - a tag to combine a prefix and multiple tags for replication configuration rule.
type And struct {
	XMLName xml.Name `xml:"And,omitempty" json:"-"`
	Prefix  string   `xml:"Prefix,omitempty" json:"Prefix,omitempty"`
	Tags    []Tag    `xml:"Tag,omitempty" json:"Tag,omitempty"`
}

// isEmpty returns true if Tags field is null
func (a And) isEmpty() bool {
	return len(a.Tags) == 0 && a.Prefix == ""
}

// Status represents Enabled/Disabled status
type Status string

// Supported status types
const (
	Enabled  Status = "Enabled"
	Disabled Status = "Disabled"
)

// DeleteMarkerReplication - whether delete markers are replicated - https://docs.aws.amazon.com/AmazonS3/latest/dev/replication-add-config.html
type DeleteMarkerReplication struct {
	Status Status `xml:"Status" json:"Status"` // should be set to "Disabled" by default
}

// IsEmpty returns true if DeleteMarkerReplication is not set
func (d DeleteMarkerReplication) IsEmpty() bool {
	return len(d.Status) == 0
}

// DeleteReplication - whether versioned deletes are replicated - this
// is a MinIO specific extension
type DeleteReplication struct {
	Status Status `xml:"Status" json:"Status"` // should be set to "Disabled" by default
}

// IsEmpty returns true if DeleteReplication is not set
func (d DeleteReplication) IsEmpty() bool {
	return len(d.Status) == 0
}

// ReplicaModifications specifies if replica modification sync is enabled
type ReplicaModifications struct {
	Status Status `xml:"Status" json:"Status"` // should be set to "Enabled" by default
}

// SourceSelectionCriteria - specifies additional source selection criteria in ReplicationConfiguration.
type SourceSelectionCriteria struct {
	ReplicaModifications ReplicaModifications `xml:"ReplicaModifications" json:"ReplicaModifications"`
}

// IsValid - checks whether SourceSelectionCriteria is valid or not.
func (s SourceSelectionCriteria) IsValid() bool {
	return s.ReplicaModifications.Status == Enabled || s.ReplicaModifications.Status == Disabled
}

// Validate source selection criteria
func (s SourceSelectionCriteria) Validate() error {
	if (s == SourceSelectionCriteria{}) {
		return nil
	}
	if !s.IsValid() {
		return fmt.Errorf("invalid ReplicaModification status")
	}
	return nil
}

// ExistingObjectReplication - whether existing object replication is enabled
type ExistingObjectReplication struct {
	Status Status `xml:"Status"` // should be set to "Disabled" by default
}

// IsEmpty returns true if DeleteMarkerReplication is not set
func (e ExistingObjectReplication) IsEmpty() bool {
	return len(e.Status) == 0
}

// Validate validates whether the status is disabled.
func (e ExistingObjectReplication) Validate() error {
	if e.IsEmpty() {
		return nil
	}
	if e.Status != Disabled && e.Status != Enabled {
		return fmt.Errorf("invalid ExistingObjectReplication status")
	}
	return nil
}

// Metrics represents inline replication metrics
// such as pending, failed and completed bytes in total for a bucket
type Metrics struct {
	// Pending size in bytes
	PendingSize uint64 `json:"pendingReplicationSize"`
	// Completed size in bytes
	ReplicatedSize uint64 `json:"completedReplicationSize"`
	// Total Replica size in bytes
	ReplicaSize uint64 `json:"replicaSize"`
	// Failed size in bytes
	FailedSize uint64 `json:"failedReplicationSize"`
	// Total number of pending operations including metadata updates
	PendingCount uint64 `json:"pendingReplicationCount"`
	// Total number of failed operations including metadata updates
	FailedCount uint64 `json:"failedReplicationCount"`
}

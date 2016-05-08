package slack

import "encoding/json"

// OutgoingMessage is used for the realtime API, and seems incomplete.
type OutgoingMessage struct {
	ID      uint64 `json:"id"`
	Channel string `json:"channel,omitempty"`
	Text    string `json:"text,omitempty"`
	Type    string `json:"type,omitempty"`
}

// Message is an auxiliary type to allow us to have a message containing sub messages
type Message struct {
	Msg
	SubMessage *Msg   `json:"message,omitempty"`
	URL        string `json:"url,omitempty"`
}

// Msg contains information about a slack message
type Msg struct {
	// Basic Message
	Type        string          `json:"type,omitempty"`
	RawChannel  json.RawMessage `json:"channel,omitempty"`
	User        string          `json:"user,omitempty"`
	Text        string          `json:"text,omitempty"`
	Timestamp   string          `json:"ts,omitempty"`
	IsStarred   bool            `json:"is_starred,omitempty"`
	PinnedTo    []string        `json:"pinned_to, omitempty"`
	Attachments []Attachment    `json:"attachments,omitempty"`
	Edited      *Edited         `json:"edited,omitempty"`

	// Message Subtypes
	SubType string `json:"subtype,omitempty"`

	// Hidden Subtypes
	Hidden           bool   `json:"hidden,omitempty"`     // message_changed, message_deleted, unpinned_item
	DeletedTimestamp string `json:"deleted_ts,omitempty"` // message_deleted
	EventTimestamp   string `json:"event_ts,omitempty"`

	// bot_message (https://api.slack.com/events/message/bot_message)
	BotID    string `json:"bot_id,omitempty"`
	Username string `json:"username,omitempty"`
	Icons    *Icon  `json:"icons,omitempty"`

	// channel_join, group_join
	Inviter string `json:"inviter,omitempty"`

	// channel_topic, group_topic
	Topic string `json:"topic,omitempty"`

	// channel_purpose, group_purpose
	Purpose string `json:"purpose,omitempty"`

	// channel_name, group_name
	Name    string `json:"name,omitempty"`
	OldName string `json:"old_name,omitempty"`

	// channel_archive, group_archive
	Members []string `json:"members,omitempty"`

	// file_share, file_comment, file_mention
	File *File `json:"file,omitempty"`

	// file_share
	Upload bool `json:"upload,omitempty"`

	// file_comment
	Comment *Comment `json:"comment,omitempty"`

	// pinned_item
	ItemType string `json:"item_type,omitempty"`

	// https://api.slack.com/rtm
	ReplyTo int    `json:"reply_to,omitempty"`
	Team    string `json:"team,omitempty"`
}

// Channel returns the channel or group for a message.
func (c *Message) Channel() string {
	if c.RawChannel == nil {
		return ""
	}

	// is it a group?
	var g Group
	if err := json.Unmarshal(c.RawChannel, &g); err == nil {
		return g.Name
	}

	return string(c.RawChannel)
}

// Group is group info.
type Group struct {
	ID         string `json:"id,omitempty"`
	Name       string `json:"name,omitempty"`
	IsGroup    bool   `json:"is_group,omitempty"`
	Created    int    `json:"created,omitempty"`
	Creator    string `json:"creator,omitempty"`
	IsArchived bool   `json:"is_archived,omitempty"`
	IsMPIM     bool   `json:"is_mpim,omitempty"`
	IsOpen     bool   `json:"is_open,omitempty"`
}

// Edited indicates that a message has been edited.
type Edited struct {
	User      string `json:"user,omitempty"`
	Timestamp string `json:"ts,omitempty"`
}

// AttachmentField contains information for an attachment field
// An Attachment can contain multiple of these
type AttachmentField struct {
	Title string `json:"title"`
	Value string `json:"value"`
	Short bool   `json:"short"`
}

// Attachment contains all the information for an attachment
type Attachment struct {
	Color    string `json:"color,omitempty"`
	Fallback string `json:"fallback"`

	AuthorName    string `json:"author_name,omitempty"`
	AuthorSubname string `json:"author_subname,omitempty"`
	AuthorLink    string `json:"author_link,omitempty"`
	AuthorIcon    string `json:"author_icon,omitempty"`

	Title     string `json:"title,omitempty"`
	TitleLink string `json:"title_link,omitempty"`
	Pretext   string `json:"pretext,omitempty"`
	Text      string `json:"text"`

	ImageURL string `json:"image_url,omitempty"`
	ThumbURL string `json:"thumb_url,omitempty"`

	Fields     []AttachmentField `json:"fields,omitempty"`
	MarkdownIn []string          `json:"mrkdwn_in,omitempty"`
}

// Icon is used for bot messages
type Icon struct {
	IconURL   string `json:"icon_url,omitempty"`
	IconEmoji string `json:"icon_emoji,omitempty"`
}

// File contains all the information for a file
type File struct {
	ID        string   `json:"id"`
	Created   JSONTime `json:"created"`
	Timestamp JSONTime `json:"timestamp"`

	Name              string `json:"name"`
	Title             string `json:"title"`
	Mimetype          string `json:"mimetype"`
	ImageExifRotation int    `json:"image_exif_rotation"`
	Filetype          string `json:"filetype"`
	PrettyType        string `json:"pretty_type"`
	User              string `json:"user"`

	Mode         string `json:"mode"`
	Editable     bool   `json:"editable"`
	IsExternal   bool   `json:"is_external"`
	ExternalType string `json:"external_type"`

	Size int `json:"size"`

	URL                string `json:"url"`          // Deprecated - never set
	URLDownload        string `json:"url_download"` // Deprecated - never set
	URLPrivate         string `json:"url_private"`
	URLPrivateDownload string `json:"url_private_download"`

	OriginalH   int    `json:"original_h"`
	OriginalW   int    `json:"original_w"`
	Thumb64     string `json:"thumb_64"`
	Thumb80     string `json:"thumb_80"`
	Thumb160    string `json:"thumb_160"`
	Thumb360    string `json:"thumb_360"`
	Thumb360Gif string `json:"thumb_360_gif"`
	Thumb360W   int    `json:"thumb_360_w"`
	Thumb360H   int    `json:"thumb_360_h"`
	Thumb480    string `json:"thumb_480"`
	Thumb480W   int    `json:"thumb_480_w"`
	Thumb480H   int    `json:"thumb_480_h"`
	Thumb720    string `json:"thumb_720"`
	Thumb720W   int    `json:"thumb_720_w"`
	Thumb720H   int    `json:"thumb_720_h"`
	Thumb960    string `json:"thumb_960"`
	Thumb960W   int    `json:"thumb_960_w"`
	Thumb960H   int    `json:"thumb_960_h"`
	Thumb1024   string `json:"thumb_1024"`
	Thumb1024W  int    `json:"thumb_1024_w"`
	Thumb1024H  int    `json:"thumb_1024_h"`

	Permalink       string `json:"permalink"`
	PermalinkPublic string `json:"permalink_public"`

	EditLink         string `json:"edit_link"`
	Preview          string `json:"preview"`
	PreviewHighlight string `json:"preview_highlight"`
	Lines            int    `json:"lines"`
	LinesMore        int    `json:"lines_more"`

	IsPublic        bool     `json:"is_public"`
	PublicURLShared bool     `json:"public_url_shared"`
	Channels        []string `json:"channels"`
	Groups          []string `json:"groups"`
	IMs             []string `json:"ims"`
	InitialComment  Comment  `json:"initial_comment"`
	CommentsCount   int      `json:"comments_count"`
	NumStars        int      `json:"num_stars"`
	IsStarred       bool     `json:"is_starred"`
}

// Comment contains all the information relative to a comment
type Comment struct {
	ID        string   `json:"id,omitempty"`
	Created   JSONTime `json:"created,omitempty"`
	Timestamp JSONTime `json:"timestamp,omitempty"`
	User      string   `json:"user,omitempty"`
	Comment   string   `json:"comment,omitempty"`
}

// User contains all the information relative to a user.
type User struct {
	ID                string      `json:"id,omitempty"`
	Name              string      `json:"name,omitempty"`
	Deleted           bool        `json:"deleted,omitempty"`
	Color             string      `json:"color,omitempty"`
	Profile           UserProfile `json:"profile,omitempty"`
	IsAdmin           bool        `json:"is_admin,omitempty"`
	IsOwner           bool        `json:"is_owner,omitempty"`
	IsPrimaryOwner    bool        `json:"is_primary_owner,omitempty"`
	IsRestricted      bool        `json:"is_restricted,omitempty"`
	IsUltraRestricted bool        `json:"is_ulra_restricted,omitempty"`
	Has2FA            bool        `json:"has_2fa,omitempty"`
	TwoFactorType     string      `json:"two_factor_type,omitempty"`
	HasFiles          bool        `json:"has_files,omitempty"`
}

// UserProfile contains all the user profile information.
type UserProfile struct {
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
	RealName  string `json:"real_name,omitempty"`
	Email     string `json:"email,omitempty"`
	Skype     string `json:"skype,omitempty"`
	Phone     string `json:"phone,omitempty"`
	Image24   string `json:"image_24,omitempty"`
	Image32   string `json:"image_32,omitempty"`
	Image48   string `json:"image_48,omitempty"`
	Image72   string `json:"image_72,omitempty"`
	Image192  string `json:"image_192,omitempty"`
}

// IM is a create IM response.
type IM struct {
	OK      bool      `json:"ok,omitempty"`
	Channel IMChannel `json:"channel,omitempty"`
	Error   string    `json:"error,omitempty"`
}

// IMChannel is a channel for IM.
type IMChannel struct {
	ID string `json:"ID,omitempty"`
}

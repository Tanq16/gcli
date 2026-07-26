package drive

import (
	"context"
	"fmt"
	"time"

	"github.com/tanq16/gcli/internal/gapi"
	driveapi "google.golang.org/api/drive/v3"
)

// Expires (relative spec) applies only to user/group grants, per the Drive API.
type ShareOptions struct {
	Emails  []string
	Anyone  bool
	Role    string
	Expires string
}

type ShareOutcome struct {
	Granted     []string
	WebViewLink string
}

type UnshareOptions struct {
	Emails []string
	Anyone bool
	All    bool
}

func ValidShareRole(role string) bool {
	switch role {
	case "reader", "commenter", "writer":
		return true
	}
	return false
}

// Hop a shortcut to its target: shortcut ACLs are immutable (§4.6).
func (c *Client) shareTarget(ctx context.Context, remoteArg string) (*driveapi.File, error) {
	f, err := c.ResolveArg(ctx, remoteArg)
	if err != nil {
		return nil, err
	}
	if IsShortcut(f) {
		if f.ShortcutDetails == nil || f.ShortcutDetails.TargetId == "" {
			return nil, usageErr("'%s' is a broken shortcut — cannot share it", f.Name)
		}
		return c.GetFile(ctx, f.ShortcutDetails.TargetId)
	}
	return f, nil
}

func (c *Client) ListShares(ctx context.Context, remoteArg string) ([]*driveapi.Permission, string, error) {
	f, err := c.shareTarget(ctx, remoteArg)
	if err != nil {
		return nil, "", err
	}
	perms, err := c.listPermissions(ctx, f.Id)
	if err != nil {
		return nil, "", err
	}
	return perms, f.WebViewLink, nil
}

func (c *Client) Share(ctx context.Context, remoteArg string, opts ShareOptions) (*ShareOutcome, error) {
	if !ValidShareRole(opts.Role) {
		return nil, usageErr("invalid --role %q (reader, commenter, writer)", opts.Role)
	}
	if len(opts.Emails) == 0 && !opts.Anyone {
		return nil, usageErr("nothing to share — pass --with <email> or --anyone")
	}
	var expires time.Time
	if opts.Expires != "" {
		if opts.Anyone && len(opts.Emails) == 0 {
			return nil, usageErr("--expires applies only to --with grants, not --anyone")
		}
		exp, err := parseExpires(opts.Expires, time.Now())
		if err != nil {
			return nil, err
		}
		expires = exp
	}

	f, err := c.shareTarget(ctx, remoteArg)
	if err != nil {
		return nil, err
	}

	var granted []string
	for _, email := range opts.Emails {
		if _, err := c.grantPermission(ctx, f.Id, "user", opts.Role, email, expires); err != nil {
			return nil, err
		}
		granted = append(granted, fmt.Sprintf("%s (%s)", email, opts.Role))
	}
	if opts.Anyone {
		if _, err := c.grantPermission(ctx, f.Id, "anyone", opts.Role, "", time.Time{}); err != nil {
			return nil, err
		}
		granted = append(granted, fmt.Sprintf("anyone with the link (%s)", opts.Role))
	}
	return &ShareOutcome{Granted: granted, WebViewLink: f.WebViewLink}, nil
}

func (c *Client) Unshare(ctx context.Context, remoteArg string, opts UnshareOptions) ([]string, error) {
	f, err := c.shareTarget(ctx, remoteArg)
	if err != nil {
		return nil, err
	}
	perms, err := c.listPermissions(ctx, f.Id)
	if err != nil {
		return nil, err
	}
	targets := matchUnshare(perms, opts)
	var removed []string
	for _, p := range targets {
		if err := c.revokePermission(ctx, f.Id, p.Id); err != nil {
			return nil, err
		}
		removed = append(removed, permLabel(p))
	}
	return removed, nil
}

// The owner grant is never revocable and is always skipped.
func matchUnshare(perms []*driveapi.Permission, opts UnshareOptions) []*driveapi.Permission {
	emails := make(map[string]bool, len(opts.Emails))
	for _, e := range opts.Emails {
		emails[e] = true
	}
	var out []*driveapi.Permission
	for _, p := range perms {
		if p.Role == "owner" {
			continue
		}
		switch {
		case opts.All:
			out = append(out, p)
		case opts.Anyone && p.Type == "anyone":
			out = append(out, p)
		case len(emails) > 0 && emails[p.EmailAddress]:
			out = append(out, p)
		}
	}
	return out
}

func permLabel(p *driveapi.Permission) string {
	who := p.EmailAddress
	if who == "" {
		who = p.Type
	}
	return fmt.Sprintf("%s (%s)", who, p.Role)
}

func (c *Client) listPermissions(ctx context.Context, fileID string) ([]*driveapi.Permission, error) {
	f, err := gapi.Retry(ctx, func() (*driveapi.File, error) {
		return c.svc.Files.Get(fileID).
			Fields("permissions(id, type, role, emailAddress, domain, displayName, allowFileDiscovery, expirationTime)").
			SupportsAllDrives(true).Context(ctx).Do()
	})
	if err != nil {
		return nil, gapi.HandleError(err)
	}
	return f.Permissions, nil
}

func (c *Client) grantPermission(ctx context.Context, fileID, permType, role, email string, expires time.Time) (*driveapi.Permission, error) {
	perm := &driveapi.Permission{Type: permType, Role: role}
	if email != "" {
		perm.EmailAddress = email
	}
	if !expires.IsZero() {
		perm.ExpirationTime = expires.UTC().Format(time.RFC3339)
	}
	created, err := gapi.Retry(ctx, func() (*driveapi.Permission, error) {
		return c.svc.Permissions.Create(fileID, perm).SupportsAllDrives(true).Context(ctx).Do()
	})
	if err != nil {
		return nil, gapi.HandleError(err)
	}
	return created, nil
}

func (c *Client) revokePermission(ctx context.Context, fileID, permID string) error {
	return gapi.RetryErr(ctx, func() error {
		return gapi.HandleError(c.svc.Permissions.Delete(fileID, permID).SupportsAllDrives(true).Context(ctx).Do())
	})
}

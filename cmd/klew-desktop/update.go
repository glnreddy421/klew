package main

import (
	"os"
	"sync"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/glnreddy421/klew/v2/internal/update"
	"github.com/glnreddy421/klew/v2/internal/version"
)

// updater holds the update downloaded and verified in this session, so the
// banner, Settings and the Help menu share one download.
type updater struct {
	mu     sync.Mutex
	staged *update.Staged
}

// UpdateStatus is the outcome of PrepareUpdate.
type UpdateStatus struct {
	Ready   bool   `json:"ready"`
	Version string `json:"version,omitempty"`
	Message string `json:"message"`
	Error   string `json:"error,omitempty"`
}

// CheckForUpdate compares the running build to the latest GitHub release.
// It never downloads anything; PrepareUpdate does.
func (a *App) CheckForUpdate() update.Result {
	return update.Check(a.baseContext(), version.Version, "", "")
}

// PrepareUpdate downloads the latest release and verifies it (checksum, the
// same Apple Developer ID team as this app, notarization, version). The
// installed app is not touched until RestartToUpdate.
func (a *App) PrepareUpdate() UpdateStatus {
	a.updates.mu.Lock()
	defer a.updates.mu.Unlock()

	// Ask GitHub again rather than trusting URLs passed in from the UI.
	res := a.CheckForUpdate()
	if res.Error != "" {
		return UpdateStatus{Message: "Could not check for updates.", Error: res.Error}
	}
	if !res.UpdateAvailable {
		return UpdateStatus{Message: res.Message}
	}
	if !res.CanSelfUpdate {
		return UpdateStatus{Message: "Klew can't update itself here.", Error: res.SelfUpdateNote}
	}
	if s := a.updates.staged; s != nil && s.Version == res.LatestVersion {
		return UpdateStatus{Ready: true, Version: s.Version, Message: "Klew " + s.Version + " is ready to install."}
	}
	staged, err := update.Prepare(a.baseContext(), res)
	if err != nil {
		a.updates.staged = nil
		return UpdateStatus{Message: "Could not download the update.", Error: err.Error()}
	}
	a.updates.staged = &staged
	return UpdateStatus{Ready: true, Version: staged.Version, Message: "Klew " + staged.Version + " is ready to install."}
}

// RestartToUpdate quits Klew; a helper then swaps in the prepared update and
// opens it. It returns an error, and keeps Klew running, if nothing is ready.
func (a *App) RestartToUpdate() string {
	a.updates.mu.Lock()
	staged := a.updates.staged
	a.updates.mu.Unlock()
	if staged == nil {
		return "No update has been downloaded yet."
	}
	if err := update.Install(*staged, os.Getpid()); err != nil {
		return "Could not start the update: " + err.Error()
	}
	if a.ctx != nil {
		runtime.Quit(a.ctx)
	}
	return ""
}

func (a *App) checkForUpdateDialog() {
	if a == nil || a.ctx == nil {
		return
	}
	res := a.CheckForUpdate()
	title := "Check for Updates"
	msg := res.Message
	if res.Error != "" {
		a.errorDialog(title, res.Message+"\n\n"+res.Error)
		return
	}
	if !res.UpdateAvailable {
		_, _ = runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
			Type: runtime.InfoDialog, Title: title, Message: msg,
		})
		return
	}

	if res.CanSelfUpdate {
		choice, err := runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
			Type:          runtime.InfoDialog,
			Title:         "Update available",
			Message:       msg + "\n\nKlew downloads and verifies the update, then restarts.",
			Buttons:       []string{"Update and Restart", "Later"},
			DefaultButton: "Update and Restart",
			CancelButton:  "Later",
		})
		if err != nil || choice != "Update and Restart" {
			return
		}
		st := a.PrepareUpdate()
		if !st.Ready {
			a.errorDialog("Update failed", st.Message+"\n\n"+st.Error+"\n\nYou can download it from the release page instead.")
			runtime.BrowserOpenURL(a.ctx, res.ReleaseURL)
			return
		}
		if msg := a.RestartToUpdate(); msg != "" {
			a.errorDialog("Update failed", msg)
		}
		return
	}

	if res.Channel == update.ChannelHomebrew && res.UpgradeCommand != "" {
		msg += "\n\n" + res.UpgradeCommand
	} else if res.SelfUpdateNote != "" {
		msg += "\n\nKlew can't update itself here: " + res.SelfUpdateNote + "."
	}
	choice, err := runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
		Type:          runtime.InfoDialog,
		Title:         "Update available",
		Message:       msg,
		Buttons:       []string{"Open release", "Later"},
		DefaultButton: "Open release",
		CancelButton:  "Later",
	})
	if err != nil {
		return
	}
	if choice == "Open release" && res.ReleaseURL != "" {
		runtime.BrowserOpenURL(a.ctx, res.ReleaseURL)
	}
}

func (a *App) errorDialog(title, msg string) {
	_, _ = runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
		Type: runtime.ErrorDialog, Title: title, Message: msg,
	})
}

package main

import (
	"fmt"
	"log/slog"
	"path/filepath"

	"kapital/backend/models"
	"kapital/backend/services"
)

// installedInstance resolves a chapter id to its instance folder when the
// instance is there. It reads the disk's own report, not a preview's (#124), as
// chapterInstance does, and says whether the instance exists instead of
// refusing, so a page can show a chapter that is not installed.
func (a *App) installedInstance(chapterID string) (models.Chapter, string, bool, error) {
	chapter, ok := a.chapter(chapterID)
	if !ok {
		return models.Chapter{}, "", false, fmt.Errorf("no chapter %q", chapterID)
	}
	report, err := a.realInstances()
	if err != nil {
		return models.Chapter{}, "", false, err
	}
	if !report.Present[chapter.ID] {
		return chapter, "", false, nil
	}
	return chapter, filepath.Join(report.Dir, chapter.Instance.ID), true, nil
}

// GetRunLogs lists a chapter's game logs and crash reports, newest first, for
// the logs page (issue 155): logs/latest.log, logs/*.log.gz and
// crash-reports/*.txt of the instance's own game folder, whichever launcher
// started the run, with a mark on the logs of a run that crashed. A chapter
// that is not installed has none. The chapter id resolves through the manifest
// and the folder is the one the instance report gives, so nothing the caller
// sends names a path. Nothing is read from a file, written or kept.
func (a *App) GetRunLogs(chapterID string) ([]models.RunLog, error) {
	chapter, dir, installed, err := a.installedInstance(chapterID)
	if err != nil {
		return nil, err
	}
	if !installed {
		return []models.RunLog{}, nil
	}
	logs, err := services.ListRunLogs(dir)
	if err != nil {
		slog.Error("list run logs", "chapter", chapter.ID, "error", err)
		return nil, a.maskedError(chapter, err)
	}
	return logs, nil
}

// maskedError is a failure as the page shows it: an error from the file
// system names the absolute game folder, which carries the player's home path,
// so its text passes the redactor like the logs themselves.
func (a *App) maskedError(chapter models.Chapter, err error) error {
	redactor, rerr := a.redactor()
	if rerr != nil {
		return maskedErr{msg: chapter.Name + ": the logs could not be read", err: err}
	}
	return maskErr(chapter, redactor, err)
}

// maskErr keeps the error for errors.Is and shows only its masked text.
func maskErr(chapter models.Chapter, redactor *services.Redactor, err error) error {
	return maskedErr{msg: chapter.Name + ": " + redactor.Redact(err.Error()), err: err}
}

// maskedErr is an error whose message has passed the redactor.
type maskedErr struct {
	msg string
	err error
}

func (e maskedErr) Error() string { return e.msg }
func (e maskedErr) Unwrap() error { return e.err }

// ReadRunLog returns one chunk of one of the files GetRunLogs listed, masked
// by the redactor (home path, user and in-game names, server addresses, IPs)
// before it leaves Go: the end of the file, or, with before set to the Offset
// of the chunk shown, the stretch preceding it. The caller names a kind and a
// base name the listing would produce; Go resolves them inside that instance's
// logs or crash-reports folder only, refusing anything else, and caps what it
// reads and unpacks. The text is returned and kept nowhere, and no line of it
// goes to slog.
func (a *App) ReadRunLog(chapterID, kind, name string, before int64) (models.RunLogText, error) {
	chapter, dir, installed, err := a.installedInstance(chapterID)
	if err != nil {
		return models.RunLogText{}, err
	}
	if !installed {
		return models.RunLogText{}, fmt.Errorf("%s is not installed", chapter.Name)
	}
	redactor, err := a.redactor()
	if err != nil {
		return models.RunLogText{}, err
	}
	text, err := services.ReadRunLog(dir, kind, name, before, redactor)
	if err != nil {
		slog.Warn("read run log", "chapter", chapter.ID, "kind", kind, "error", err)
		return models.RunLogText{}, maskErr(chapter, redactor, err)
	}
	slog.Info("run log read", "chapter", chapter.ID, "kind", kind, "lines", text.Lines, "earlier", text.Truncated)
	return text, nil
}

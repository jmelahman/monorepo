package releases

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	kev "github.com/jmelahman/kev-go"

	"github.com/jmelahman/kanban/internal/db"
)

const (
	// assessCommits and assessSubject bound what the model reads of a subtree's
	// unreleased commits: the newest ones, by subject.
	assessCommits = 60
	assessSubject = 200
	// assessTimeout bounds one pass over every repo. A model on a CPU takes
	// seconds a subtree, so this is only reached by something being stuck.
	assessTimeout = 30 * time.Minute
)

// assessQuestion is what the model is asked of a subtree's unreleased commits.
// It isn't "should this be released?": against this monorepo's history, Kev-0.8B
// couldn't tell the commits that led to a release from those that didn't, while
// this question separated internal-only changes from ones users see.
var assessQuestion = kev.Question{
	Instructions: "Do these commits change anything a user of this project would notice?",
	Options: []string{
		"no: internal only, such as CI, tests, refactors, docs or formatting",
		"yes: a bug fix, a new feature or changed behaviour",
	},
}

// Model answers questions about a piece of text. It's *kev.Model in production.
type Model interface {
	Decide(ctx context.Context, state string, questions []kev.Question) ([][]float64, int, error)
	Close()
}

// OpenModel loads a model, which can take gigabytes until it's closed.
type OpenModel func() (Model, error)

// OpenKev returns an OpenModel for a Kev checkpoint. base is the backbone an
// adapter was trained on; a full-weight checkpoint needs none.
func OpenKev(checkpoint, base string) OpenModel {
	return func() (Model, error) {
		// Each state gets one question, so there's nothing to reuse.
		m, err := kev.Open(checkpoint, kev.Options{Base: base, PrefixCache: -1})
		if err != nil {
			return nil, err
		}
		return m, nil
	}
}

// Assessment is a model's read of a subtree's unreleased commits.
type Assessment struct {
	// Visible is the probability that users would notice them.
	Visible float64 `json:"visible"`
}

// AssessmentStatus says assessments are on, and how the latest pass went.
type AssessmentStatus struct {
	Running bool   `json:"running"`
	Error   string `json:"error,omitempty"`
}

// EnableAssessment turns on assessing unreleased commits with what open loads.
// Call it before serving. model names what open loads, so that changing it
// assesses everything again. Nothing is loaded until there's something to
// assess, and it's closed again once that's done.
func (s *Service) EnableAssessment(model string, open OpenModel) {
	saved, err := s.store.ListReleaseAssessments(s.ctx)
	if err != nil {
		log.Printf("releases: load saved assessments: %v", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.modelName, s.openModel = model, open
	s.assessments = map[string]map[string]db.ReleaseAssessment{}
	for _, a := range saved {
		s.setAssessment(a)
	}
}

func (s *Service) setAssessment(a db.ReleaseAssessment) {
	if s.assessments[a.RepoPath] == nil {
		s.assessments[a.RepoPath] = map[string]db.ReleaseAssessment{}
	}
	s.assessments[a.RepoPath][a.Prefix] = a
}

// startAssessment assesses, in the background, the repos refreshed since their
// last pass. Like CI, it only starts when something reads the summary, so an
// idle server never loads a model.
func (s *Service) startAssessment(repos []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.openModel == nil || s.assessing || s.ctx.Err() != nil {
		return
	}
	var due []Project
	for _, repo := range repos {
		p, ok := s.projects[repo]
		if ok && !s.refreshing[repo] && !s.assessed[repo] {
			s.assessed[repo] = true
			due = append(due, p)
		}
	}
	if len(due) == 0 {
		return
	}
	s.assessing = true
	s.wg.Go(func() {
		err := s.assess(due)
		s.mu.Lock()
		defer s.mu.Unlock()
		s.assessing = false
		s.assessError = ""
		if err != nil {
			s.assessError = redact(err.Error())
		}
	})
}

// assessJob is a subtree whose commits have no assessment yet.
type assessJob struct {
	repo, prefix, latest string
	commits              int
	state, input         string
}

// assess works out what's new in projects and runs the model over only that,
// returning the first failure. A failure is retried after the next refresh.
func (s *Service) assess(projects []Project) error {
	ctx, cancel := context.WithTimeout(s.ctx, assessTimeout)
	defer cancel()
	var jobs []assessJob
	var failed error
	for _, p := range projects {
		found, err := s.assessJobs(ctx, p)
		failed = errors.Join(failed, err)
		jobs = append(jobs, found...)
	}
	if len(jobs) == 0 {
		return failed
	}
	s.mu.Lock()
	openModel := s.openModel
	s.mu.Unlock()
	model, err := openModel()
	if err != nil {
		return fmt.Errorf("load model: %w", err)
	}
	defer func() {
		model.Close()
		// The model's memory is the reason this is opt-in, so hand it back now
		// rather than when the runtime next gets to it.
		debug.FreeOSMemory()
	}()
	for _, j := range jobs {
		probs, _, err := model.Decide(ctx, j.state, []kev.Question{assessQuestion})
		if err != nil {
			failed = errors.Join(failed, fmt.Errorf("assess %s: %w", j.prefix, err))
			if ctx.Err() != nil {
				return failed
			}
			continue
		}
		a := db.ReleaseAssessment{RepoPath: j.repo, Prefix: j.prefix, Input: j.input, Latest: j.latest, Commits: j.commits, Visible: probs[0][1]}
		if err := s.store.SaveReleaseAssessment(ctx, a); err != nil {
			log.Printf("releases: save assessment of %s: %v", j.prefix, err)
		}
		s.mu.Lock()
		s.setAssessment(a)
		s.mu.Unlock()
	}
	return failed
}

// assessJobs returns the subtrees of p with commits since a release that haven't
// been assessed as they are now. A subtree never released is skipped: it has a
// whole history rather than a set of changes.
func (s *Service) assessJobs(ctx context.Context, p Project) ([]assessJob, error) {
	o, err := open(ctx, p.RepoPath, nil)
	if err != nil {
		return nil, err
	}
	var jobs []assessJob
	var failed error
	for _, sub := range p.Subtrees {
		if p.Rev == "" || sub.Error != "" || !sub.Counted || sub.Latest == "" || sub.Unreleased == 0 {
			continue
		}
		revs := "refs/tags/" + sub.Prefix + "/" + sub.Latest + ".." + p.Rev
		subjects, err := o.Repo.Output("log", "--format=%s", "-n", strconv.Itoa(assessCommits), revs, "--", sub.Prefix)
		if err != nil {
			failed = errors.Join(failed, fmt.Errorf("read commits of %s: %w", sub.Prefix, err))
			continue
		}
		state := assessState(sub, strings.Split(strings.TrimSpace(subjects), "\n"))
		s.mu.Lock()
		input := assessInput(s.modelName, state)
		known := s.assessments[p.RepoPath][sub.Prefix]
		s.mu.Unlock()
		if known.Input == input {
			continue
		}
		jobs = append(jobs, assessJob{p.RepoPath, sub.Prefix, sub.Latest, sub.Unreleased, state, input})
	}
	return jobs, failed
}

// assessState is the text the model reads. It leaves out anything that changes
// without the commits changing, such as how long ago the release was, so an
// assessment holds until there's something new to read.
func assessState(sub SubtreeState, subjects []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Project: %s\nLatest release: %s\nCommits since that release: %d\n", sub.Prefix, sub.Latest, sub.Unreleased)
	for _, subject := range subjects {
		if len(subject) > assessSubject {
			subject = subject[:assessSubject]
		}
		b.WriteString("- " + subject + "\n")
	}
	return strings.TrimSuffix(b.String(), "\n")
}

// assessInput identifies everything an assessment's answer depends on.
func assessInput(model, state string) string {
	h := sha256.New()
	for _, part := range append([]string{model, state, assessQuestion.Instructions}, assessQuestion.Options...) {
		fmt.Fprintf(h, "%d:%s", len(part), part)
	}
	return hex.EncodeToString(h.Sum(nil))
}

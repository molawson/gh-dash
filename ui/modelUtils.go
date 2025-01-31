package ui

import (
	"bytes"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"text/template"

	"github.com/dlvhdr/gh-dash/data"
	"github.com/dlvhdr/gh-dash/ui/components/section"
)

func (m *Model) getCurrSection() section.Section {
	sections := m.getCurrentViewSections()
	if len(sections) == 0 || m.currSectionId >= len(sections) {
		return nil
	}
	return sections[m.currSectionId]
}

func (m *Model) getCurrRowData() data.RowData {
	section := m.getCurrSection()
	if section == nil {
		return nil
	}
	return section.GetCurrRow()
}

func (m *Model) getSectionAt(id int) section.Section {
	sections := m.getCurrentViewSections()
	if len(sections) <= id {
		return nil
	}
	return sections[id]
}

func (m *Model) getPrevSectionId() int {
	sectionsConfigs := m.ctx.GetViewSectionsConfig()
	m.currSectionId = (m.currSectionId - 1) % len(sectionsConfigs)
	if m.currSectionId < 0 {
		m.currSectionId += len(sectionsConfigs)
	}

	return m.currSectionId
}

func (m *Model) getNextSectionId() int {
	return (m.currSectionId + 1) % len(m.ctx.GetViewSectionsConfig())
}

// support [user|org]/* matching for repositories
// and local path mapping to [partial path prefix]/*
// prioritize full repo mapping if it exists
func getRepoLocalPath(repoName string, cfgPaths map[string]string ) string {
	exactMatchPath, ok := cfgPaths[repoName]
	// prioritize full repo to path mapping in config
	if ok { return exactMatchPath	}

	var repoPath string

	owner, repo, repoValid := func() (string, string, bool) {
		repoParts := strings.Split(repoName, "/")
		// return repo owner, repo, and indicate properly owner/repo format
		return repoParts[0], repoParts[len(repoParts)-1], len(repoParts) == 2
	}()

	if repoValid {
		// match config:repoPath values of {owner}/* as map key
		wildcardPath, wildcarded := cfgPaths[fmt.Sprintf("%s/*", owner)]

		if wildcarded {
			// adjust wildcard match to wildcard path - ~/somepath/* to ~/somepath/{repo}
			repoPath = fmt.Sprintf("%s/%s", strings.TrimSuffix(wildcardPath, "/*"), repo)
		}
	}

	return repoPath
}

type CommandTemplateInput struct {
	RepoName    string
	RepoPath    string
	PrNumber    int
	HeadRefName string
}

func (m *Model) executeKeybinding(key string) {
	currRowData := m.getCurrRowData()
	for _, keybinding := range m.ctx.Config.Keybindings.Prs {
		if keybinding.Key != key {
			continue
		}

		switch data := currRowData.(type) {
		case *data.PullRequestData:
			m.runCustomCommand(keybinding.Command, data)
		}
	}
}

func (m *Model) runCustomCommand(commandTemplate string, prData *data.PullRequestData) {
	cmd, err := template.New("keybinding_command").Parse(commandTemplate)
	if err != nil {
		log.Fatal(err)
	}
	repoName := prData.GetRepoNameWithOwner()
	repoPath := getRepoLocalPath(repoName, m.ctx.Config.RepoPaths)

	var buff bytes.Buffer
	err = cmd.Execute(&buff, CommandTemplateInput{
		RepoName:    repoName,
		RepoPath:    repoPath,
		PrNumber:    prData.Number,
		HeadRefName: prData.HeadRefName,
	})
	if err != nil {
		log.Fatal(err)
	}

	job := exec.Command("/bin/sh", "-c", buff.String())
	out, err := job.CombinedOutput()
	if err != nil {
		log.Fatalf("Got an error while executing command: %s. \nError: %v\n%s", job, err, out)
	}
}

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"text/template"

	"github.com/russross/blackfriday"
)

const layout = `
<!DOCTYPE html>
<html lang="en">
  <head>
    <title>Changelog</title>
  </head>
  <body>
	  {{.Content}}
	</body>
</html>
`

// ChangeLog represents a collection of changes between versions
type ChangeLog struct {
	NameTags string
	Commits  []CommitMessage
}

// CommitMessage contains the commit information
type CommitMessage struct {
	ShortCommit string `json:"shortcommit"`
	Commit      string `json:"commit"`
	Author      string `json:"author"`
	Email       string `json:"email"`
	Date        string `json:"date"`
	Message     string `json:"message"`
}

// Template for html format
type Template struct {
	Content string
}

// Report full report structure
type Report struct {
	pattern   string
	originURL string
	ChangeLog []ChangeLog
	JSON      string
	Markdown  string
	HTML      string
}

func fetchTags(releasePattern string) []string {
	out, _ := exec.Command("git", "log", "--tags", "--simplify-by-decoration", "--pretty=\"%ai @%d\"").Output()
	uncuratedTags := strings.Split(string(out), "\n")

	var tags []string
	validTag := regexp.MustCompile(fmt.Sprintf(`(.*\s)@\s.*tag:\s(%v)`, releasePattern))

	var match [][]string
	for _, tag := range uncuratedTags {
		match = validTag.FindAllStringSubmatch(tag, -1)
		if len(match) > 0 && len(match[0]) > 2 {
			// fmt.Printf("%v => %v \n", match[0][1], match[0][2])
			tags = append(tags, strings.Replace(match[0][2], ",", "", -1))
		}
	}
	return tags
}

func fetchChanges(releasePattern string) []ChangeLog {
	var changeLogs []ChangeLog
	tags := fetchTags(releasePattern)

	var diffs []string
	for i := 0; i < len(tags)-1; i++ {
		diffs = append(diffs, fmt.Sprintf("%v..%v", tags[i+1], tags[i]))
	}

	format := "--pretty=format:{\"shortcommit\":\"%h\", \"commit\":\"%H\", \"author\":\"%an\", \"email\":\"%ae\", \"date\":\"%ad\", \"message\":\"%f\"},"

	var out []byte
	var outs string
	var data []CommitMessage
	re := regexp.MustCompile(`,$`)
	for _, diff := range diffs {
		out, _ = exec.Command("git", "log", format, diff).Output()
		outs = fmt.Sprintf("[%v]", re.ReplaceAllString(string(out), ""))
		// fmt.Printf("%v\n", outs)
		if err := json.Unmarshal([]byte(outs), &data); err != nil {
			panic(err)
		}
		changeLogs = append(changeLogs, ChangeLog{NameTags: diff, Commits: data})
	}

	return changeLogs
}

func (c *CommitMessage) toMarkdown(originURL string) string {
	space := regexp.MustCompile(`-`)
	commitURL := fmt.Sprintf("%vcommit/%v", originURL, c.Commit)
	return fmt.Sprintf("* [%v](%v) %v [%v](mailto:%v)\n", c.ShortCommit, commitURL, space.ReplaceAllString(c.Message, " "), c.Author, c.Email)
}

func fetchProjectURL() string {
	originURL, _ := exec.Command("git", "config", "--get", "remote.origin.url").Output()
	re := regexp.MustCompile(`:`)
	originURL = []byte(re.ReplaceAllString(string(originURL), `/`))
	re = regexp.MustCompile(`\.git\n$`)
	originURL = []byte(re.ReplaceAllString(string(originURL), `/`))
	re = regexp.MustCompile(`^git@`)
	originURL = []byte(re.ReplaceAllString(string(originURL), `https://`))

	return string(originURL)
}

func buildReport(releasePattern string) Report {
	report := Report{pattern: releasePattern, originURL: fetchProjectURL()}
	report.ChangeLog = fetchChanges(releasePattern)

	byteJSON, _ := json.Marshal(report.ChangeLog)
	report.JSON = string(byteJSON)

	return report
}

func (r *Report) getMarkdown() string {
	if len(r.Markdown) > 0 {
		return r.Markdown
	}
	var mdBuffer bytes.Buffer
	mdBuffer.WriteString("# Changelog\n")

	for _, change := range r.ChangeLog {
		mdBuffer.WriteString(fmt.Sprintf("\n## %v \n\n", change.NameTags))
		for _, commit := range change.Commits {
			mdBuffer.WriteString(commit.toMarkdown(string(r.originURL)))
		}
	}
	r.Markdown = mdBuffer.String()
	return r.Markdown
}

func (r *Report) getHTML() string {
	if len(r.HTML) > 0 {
		return r.HTML
	}
	out := bytes.NewBuffer(nil)

	html := blackfriday.MarkdownCommon([]byte(r.getMarkdown()))

	t := template.Must(template.New("layout").Parse(layout))
	err := t.Execute(out, Template{Content: string(html)})
	if err != nil {
		panic(err)
	}
	r.HTML = out.String()
	return r.HTML
}

func main() {
	releasePattern := `v[\d{1,4}\.]{1,}`
	// releasePattern := `release-v?[\d{1,4}\.]{1,}`
	report := buildReport(releasePattern)
	fmt.Println(report.JSON)
	fmt.Println(report.getMarkdown())
	fmt.Print(report.getHTML())
}

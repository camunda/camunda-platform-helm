// Copyright 2026 Camunda Services GmbH
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

package main

import (
	"fmt"
	"strings"
)

type projectItem struct {
	ID      string
	Project struct {
		ID     string
		Number int
	}
	FieldValues struct {
		Nodes []struct {
			Name  string
			Field struct{ Name string }
		}
	}
}

func (p projectItem) status() string {
	for _, f := range p.FieldValues.Nodes {
		if f.Field.Name == "Status" {
			return f.Name
		}
	}
	return ""
}

func (c *githubClient) projectItem(number, project int) (*projectItem, error) {
	parts := strings.Split(c.repo, "/")
	cursor := ""
	for {
		var data struct {
			Repository struct {
				Issue struct {
					ProjectItems struct {
						Nodes    []projectItem
						PageInfo struct {
							HasNextPage bool
							EndCursor   string
						}
					}
				}
			}
		}
		query := `query($owner:String!,$repo:String!,$number:Int!,$cursor:String){repository(owner:$owner,name:$repo){issue(number:$number){projectItems(first:100,after:$cursor){nodes{id project{id number} fieldValues(first:100){nodes{... on ProjectV2ItemFieldSingleSelectValue{name field{... on ProjectV2SingleSelectField{name}}}}}} pageInfo{hasNextPage endCursor}}}}}`
		vars := map[string]any{"owner": parts[0], "repo": parts[1], "number": number, "cursor": nil}
		if cursor != "" {
			vars["cursor"] = cursor
		}
		if err := c.graphql(query, vars, &data); err != nil {
			return nil, err
		}
		items := data.Repository.Issue.ProjectItems
		for _, item := range items.Nodes {
			if item.Project.Number == project {
				return &item, nil
			}
		}
		if !items.PageInfo.HasNextPage {
			return nil, nil
		}
		if items.PageInfo.EndCursor == "" || items.PageInfo.EndCursor == cursor {
			return nil, fmt.Errorf("invalid project item cursor")
		}
		cursor = items.PageInfo.EndCursor
	}
}

func shouldBacklog(internal bool, status string) bool {
	return !internal || status == "" || status == "📥 Inbox"
}

func (c *githubClient) backlog(number, project int, internal bool) error {
	i, err := c.getIssue(number)
	if err != nil {
		return err
	}
	if i.State != "open" {
		return nil
	}
	item, err := c.projectItem(number, project)
	if err != nil {
		return err
	}
	if item == nil || !shouldBacklog(internal, item.status()) {
		return nil
	}
	var data struct {
		Node struct {
			Field struct {
				ID      string
				Options []struct{ ID, Name string }
			}
		}
	}
	query := `query($id:ID!){node(id:$id){... on ProjectV2{field(name:"Status"){... on ProjectV2SingleSelectField{id options{id name}}}}}}`
	if err := c.graphql(query, map[string]any{"id": item.Project.ID}, &data); err != nil {
		return err
	}
	field := data.Node.Field
	for _, option := range field.Options {
		if option.Name == "📋 Backlog" && field.ID != "" {
			mutation := `mutation($input:UpdateProjectV2ItemFieldValueInput!){updateProjectV2ItemFieldValue(input:$input){projectV2Item{id}}}`
			return c.graphql(mutation, map[string]any{"input": map[string]any{"projectId": item.Project.ID, "itemId": item.ID, "fieldId": field.ID, "value": map[string]string{"singleSelectOptionId": option.ID}}}, nil)
		}
	}
	return fmt.Errorf("project %d has no Backlog status option", project)
}

func (c *githubClient) candidates(project int) ([]int, error) {
	numbers := []int{}
	for page := 1; ; page++ {
		var issues []issue
		path := fmt.Sprintf("/repos/%s/issues?state=open&per_page=100&page=%d", c.repo, page)
		if err := c.request("GET", path, nil, &issues); err != nil {
			return nil, err
		}
		for _, i := range issues {
			if i.isPullRequest() {
				continue
			}
			internal, err := c.internal(i)
			if err != nil {
				return nil, err
			}
			item, err := c.projectItem(i.Number, project)
			if err != nil {
				return nil, err
			}
			if item == nil {
				continue
			}
			assess := i.assessable() && (item.status() == "📥 Inbox" || internal && item.status() == "📋 Backlog")
			intake := !internal && !i.has(slackNotified) && (i.has(slackRetry) || i.has(slackPending) || i.has("triage:manual") && item.status() == "📥 Inbox")
			if assess || intake {
				numbers = append(numbers, i.Number)
			}
		}
		if len(issues) < 100 {
			break
		}
	}
	if len(numbers) > 256 {
		return nil, fmt.Errorf("%d candidates exceed the Actions matrix limit; process issues individually first", len(numbers))
	}
	return numbers, nil
}

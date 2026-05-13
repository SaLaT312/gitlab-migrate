package main

import (
	"fmt"

	gitlab "gitlab.com/gitlab-org/api/client-go"
)

type GroupsPath []GroupsPathData

type GroupsPathData struct {
	Name string
	Path string
}

func CreateGroup(git *gitlab.Client, parentGroupName string, groups GroupsPath) error {
	parentGroupID := findGroupID(git, parentGroupName)
	if parentGroupID == 0 {
		return fmt.Errorf("group '%s' not found", parentGroupName)
	}

	for _, group := range groups {
		groupID, err := createOrFindGroup(git, group.Name, group.Path, parentGroupID)
		if err != nil {
			return fmt.Errorf("error creating group '%s': %w", group.Name, err)
		}
		parentGroupID = groupID
	}
	return nil
}

func findGroupID(client *gitlab.Client, path string) int {
	group, _, err := client.Groups.GetGroup(path, nil)
	if err != nil {
		return 0
	}
	return group.ID
}

func createOrFindGroup(client *gitlab.Client, name, path string, parentID int) (int, error) {
	groupID := findGroupID(client, fmt.Sprintf("%s/%s", pathFromID(client, parentID), path))
	if groupID != 0 {
		return groupID, nil
	}

	groupOptions := &gitlab.CreateGroupOptions{
		Name:       gitlab.Ptr(name),
		Path:       gitlab.Ptr(path),
		ParentID:   gitlab.Ptr(parentID),
		Visibility: gitlab.Ptr(gitlab.PrivateVisibility),
	}
	group, _, err := client.Groups.CreateGroup(groupOptions)
	if err != nil {
		return 0, err
	}
	return group.ID, nil
}

func pathFromID(client *gitlab.Client, groupID int) string {
	group, _, err := client.Groups.GetGroup(groupID, nil)
	if err != nil {
		return ""
	}
	return group.FullPath
}

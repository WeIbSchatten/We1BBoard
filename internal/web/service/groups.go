package service

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/we1bboard/we1bboard/internal/database"
	"github.com/we1bboard/we1bboard/internal/database/model"
)

// GroupSummary is the list DTO for client groups.
type GroupSummary struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
	Up    int64  `json:"up"`
	Down  int64  `json:"down"`
}

func loadGroupMarkers() []string {
	raw := database.GetSetting("clientGroups")
	if raw == "" {
		return nil
	}
	var names []string
	if err := json.Unmarshal([]byte(raw), &names); err != nil {
		return nil
	}
	out := make([]string, 0, len(names))
	seen := map[string]bool{}
	for _, n := range names {
		n = strings.TrimSpace(n)
		if n == "" || seen[n] {
			continue
		}
		seen[n] = true
		out = append(out, n)
	}
	return out
}

func saveGroupMarkers(names []string) error {
	b, err := json.Marshal(names)
	if err != nil {
		return err
	}
	return database.SetSetting("clientGroups", string(b))
}

func (s *ClientService) ListGroups() ([]GroupSummary, error) {
	type row struct {
		Name  string
		Count int
		Up    int64
		Down  int64
	}
	var rows []row
	err := database.DB.Model(&model.Client{}).
		Select("group_name as name, count(*) as count, coalesce(sum(up),0) as up, coalesce(sum(down),0) as down").
		Where("group_name <> '' AND group_name IS NOT NULL").
		Group("group_name").
		Order("group_name asc").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	byName := map[string]GroupSummary{}
	for _, r := range rows {
		byName[r.Name] = GroupSummary{Name: r.Name, Count: r.Count, Up: r.Up, Down: r.Down}
	}
	for _, m := range loadGroupMarkers() {
		if _, ok := byName[m]; !ok {
			byName[m] = GroupSummary{Name: m}
		}
	}
	out := make([]GroupSummary, 0, len(byName))
	for _, g := range byName {
		out = append(out, g)
	}
	// stable-ish order by name
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j].Name < out[i].Name {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out, nil
}

func (s *ClientService) CreateGroup(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("name required")
	}
	if len(name) > 128 {
		return fmt.Errorf("name too long")
	}
	markers := loadGroupMarkers()
	for _, m := range markers {
		if m == name {
			return nil
		}
	}
	markers = append(markers, name)
	return saveGroupMarkers(markers)
}

func (s *ClientService) RenameGroup(oldName, newName string) (int64, error) {
	oldName = strings.TrimSpace(oldName)
	newName = strings.TrimSpace(newName)
	if oldName == "" || newName == "" {
		return 0, fmt.Errorf("oldName and newName required")
	}
	if len(newName) > 128 {
		return 0, fmt.Errorf("newName too long")
	}
	res := database.DB.Model(&model.Client{}).Where("group_name = ?", oldName).Update("group_name", newName)
	if res.Error != nil {
		return 0, res.Error
	}
	markers := loadGroupMarkers()
	next := make([]string, 0, len(markers))
	seen := map[string]bool{}
	for _, m := range markers {
		if m == oldName {
			m = newName
		}
		if seen[m] {
			continue
		}
		seen[m] = true
		next = append(next, m)
	}
	_ = saveGroupMarkers(next)
	return res.RowsAffected, nil
}

func (s *ClientService) DeleteGroup(name string) (int64, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return 0, fmt.Errorf("name required")
	}
	res := database.DB.Model(&model.Client{}).Where("group_name = ?", name).Update("group_name", "")
	if res.Error != nil {
		return 0, res.Error
	}
	markers := loadGroupMarkers()
	next := make([]string, 0, len(markers))
	for _, m := range markers {
		if m != name {
			next = append(next, m)
		}
	}
	_ = saveGroupMarkers(next)
	return res.RowsAffected, nil
}

func (s *ClientService) AssignGroup(name string, ids []uint) (int64, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return 0, fmt.Errorf("name required")
	}
	if len(ids) == 0 {
		return 0, fmt.Errorf("ids required")
	}
	_ = s.CreateGroup(name)
	res := database.DB.Model(&model.Client{}).Where("id IN ?", ids).Update("group_name", name)
	return res.RowsAffected, res.Error
}

func (s *ClientService) UnassignGroup(ids []uint) (int64, error) {
	if len(ids) == 0 {
		return 0, fmt.Errorf("ids required")
	}
	res := database.DB.Model(&model.Client{}).Where("id IN ?", ids).Update("group_name", "")
	return res.RowsAffected, res.Error
}

func (s *ClientService) ResetGroupTraffic(name string) (int64, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return 0, fmt.Errorf("name required")
	}
	res := database.DB.Model(&model.Client{}).Where("group_name = ?", name).Updates(map[string]any{"up": 0, "down": 0})
	return res.RowsAffected, res.Error
}

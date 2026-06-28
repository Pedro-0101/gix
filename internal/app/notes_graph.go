package app

// GraphNode is one note in the graph.
type GraphNode struct {
	ID    int64    `json:"id"`
	Title string   `json:"title"`
	Tags  []string `json:"tags"`
}

// GraphEdge is one directed link between notes.
type GraphEdge struct {
	Source int64 `json:"source"`
	Target int64 `json:"target"`
}

// GraphData is the full graph returned to the frontend.
type GraphData struct {
	Nodes []GraphNode `json:"nodes"`
	Edges []GraphEdge `json:"edges"`
}

// GetGraphData returns every note (as a node) and edges between notes that share
// at least one tag. For each tag, all pairs of notes with that tag are linked.
func (s *NotesService) GetGraphData() (GraphData, error) {
	var data GraphData
	if s.db == nil {
		return data, nil
	}
	notes, err := s.db.ListNotes()
	if err != nil {
		return data, err
	}
	for _, n := range notes {
		data.Nodes = append(data.Nodes, GraphNode{ID: n.ID, Title: n.Title, Tags: n.Tags})
	}

	// Build tag → note IDs map.
	tagToNotes := map[string][]int64{}
	for _, n := range notes {
		for _, tag := range n.Tags {
			tagToNotes[tag] = append(tagToNotes[tag], n.ID)
		}
	}

	// Create edges for every pair sharing a tag.
	seen := map[[2]int64]bool{}
	for _, ids := range tagToNotes {
		if len(ids) < 2 {
			continue
		}
		for i := 0; i < len(ids); i++ {
			for j := i + 1; j < len(ids); j++ {
				a, b := ids[i], ids[j]
				if a > b {
					a, b = b, a
				}
				key := [2]int64{a, b}
				if !seen[key] {
					seen[key] = true
					data.Edges = append(data.Edges, GraphEdge{Source: a, Target: b})
				}
			}
		}
	}
	return data, nil
}

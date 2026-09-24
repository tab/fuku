package environment

import "fuku/internal/model"

// merge reads the files in dir in order, later files overriding earlier keys (a file the reader cannot read is skipped)
func (s *Store) merge(dir string, files []string) []model.Env {
	if len(files) == 0 {
		return nil
	}

	indices := make(map[string]int)

	var result []model.Env

	for _, name := range files {
		entries, err := s.reader.Read(dir, name)
		if err != nil {
			continue
		}

		for _, e := range entries {
			idx, exists := indices[e.Key]
			if exists {
				result[idx].Value = e.Value

				continue
			}

			indices[e.Key] = len(result)
			result = append(result, model.Env{Key: e.Key, Value: e.Value})
		}
	}

	return result
}

package db

import "fmt"

// Feed pagination cursors are opaque to the client. Two shapes exist because
// the two ranking strategies paginate differently: default scoring uses a
// keyset on (flat_score, id), custom scoring paginates a pre-sorted
// in-memory window by offset.

func encodeScoreCursor(score, id int) string {
	return fmt.Sprintf("s:%d:%d", score, id)
}

func decodeScoreCursor(cursor string) (score, id int, err error) {
	if _, err := fmt.Sscanf(cursor, "s:%d:%d", &score, &id); err != nil {
		return 0, 0, fmt.Errorf("parsing score cursor %q: %w", cursor, err)
	}
	return score, id, nil
}

func encodeOffsetCursor(offset int) string {
	return fmt.Sprintf("o:%d", offset)
}

func decodeOffsetCursor(cursor string) (int, error) {
	var offset int
	if _, err := fmt.Sscanf(cursor, "o:%d", &offset); err != nil {
		return 0, fmt.Errorf("parsing offset cursor %q: %w", cursor, err)
	}
	return offset, nil
}

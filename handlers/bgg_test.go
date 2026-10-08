package handlers

import (
	"encoding/xml"
	"testing"
)

func TestBGGExpansionDetectionUsesItemType(t *testing.T) {
	tests := []struct {
		name      string
		xml       string
		expansion bool
	}{
		{
			name:      "base game with an expansion is not an expansion",
			xml:       `<items><item type="boardgame" id="1"><link type="boardgameexpansion" id="2" value="Expansion for Base-game"/></item></items>`,
			expansion: false,
		},
		{
			name:      "expansion item is an expansion",
			xml:       `<items><item type="boardgameexpansion" id="2"><link type="boardgame" id="1" value="Base-game"/></item></items>`,
			expansion: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var response BGGThingResponse
			if err := xml.Unmarshal([]byte(test.xml), &response); err != nil {
				t.Fatalf("failed to parse BGG fixture: %v", err)
			}
			if len(response.Items) != 1 {
				t.Fatalf("expected one BGG item, got %d", len(response.Items))
			}

			got := isBGGExpansion(response.Items[0])
			if got != test.expansion {
				t.Errorf("isExpansion = %t, want %t", got, test.expansion)
			}
		})
	}
}

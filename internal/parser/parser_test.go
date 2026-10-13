package parser

import (
	"strings"
	"testing"
)

func TestCleanWikitext(t *testing.T) {
	raw := `
{{Stub|Cinematic Cutscenes not placed in yet.}}
{{Dialogue Start}}
{{WavesLine|text = :'''Unknown Sender:''' Grand Opening Special! Reply "Unsub" to unsubscribe.
:{Choice} But I haven't tried it yet.
::'''(Rover):''' But I haven't tried it yet.}}
----
;Head to the tea stall in Xianling Village
:'''Abby:''' Say, on this trip to the village, we're not having that same stuff again, are we?
:'''{{Rover}}:''' ...Mengzhou Fish in Vinegar?
:'''Abby:''' Eugh, yeah. Besides that, Xuanfang's full of good eats.
:'''Narrator:''' Determined to defeat the {{Rubi|Threnodian|Ovathrax}} once and for all.
`

	cleaned := CleanWikitext(raw)

	if !strings.Contains(cleaned, "Threnodian (Ovathrax)") {
		t.Errorf("Expected Rubi template expansion 'Threnodian (Ovathrax)', got: %s", cleaned)
	}

	if strings.Contains(cleaned, "{{Stub") {
		t.Errorf("Cleaned text should not contain {{Stub, got: %s", cleaned)
	}
	if strings.Contains(cleaned, "{{Rover}}") {
		t.Errorf("Cleaned text should have replaced {{Rover}} with Rover, got: %s", cleaned)
	}
	if !strings.Contains(cleaned, "Rover: ...Mengzhou Fish in Vinegar?") {
		t.Errorf("Expected Rover dialogue in cleaned text, got: %s", cleaned)
	}
	if !strings.Contains(cleaned, "[Objective: Head to the tea stall in Xianling Village]") {
		t.Errorf("Expected parsed objective marker, got: %s", cleaned)
	}

	speakers := ExtractSpeakers(cleaned)
	expectedSpeakers := []string{"Unknown Sender", "Rover", "Abby"}
	for _, expected := range expectedSpeakers {
		found := false
		for _, s := range speakers {
			if s == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected speaker %q in %v", expected, speakers)
		}
	}
}

func TestParseIntoChunks(t *testing.T) {
	raw := `
==Steps==
;Talk to Abby
1. Go to the village.
2. Defeat the Tacet Discords.

==Dialogue==
;Head to the tea stall in Xianling Village
:'''Abby:''' Let's grab some food!
:'''Rover:''' Good idea.

;(Cutscene plays)
:'''Narrator:''' Ovathrax, the dreadful Threnodian monster, fed on the frequencies of weapons...

;(Cutscene ends)

==Gallery==
[[File:Pic.png|thumb|An image]]
`

	chunks := ParseIntoChunks("Rewinding Raindrops", raw, 500, 50)
	if len(chunks) == 0 {
		t.Fatalf("Expected chunks to be generated, got 0")
	}

	foundOvathrax := false
	for _, c := range chunks {
		if strings.Contains(c.Content, "Ovathrax") {
			foundOvathrax = true
			break
		}
	}
	if !foundOvathrax {
		t.Errorf("Expected chunk containing Ovathrax dialogue to be retained")
	}

	// Gallery should be omitted
	for _, c := range chunks {
		if strings.EqualFold(c.Section, "Gallery") {
			t.Errorf("Section Gallery should have been skipped, but found chunk: %+v", c)
		}
	}

	foundDialogue := false
	for _, c := range chunks {
		if c.Section == "Dialogue" {
			foundDialogue = true
			if len(c.Speakers) == 0 {
				t.Errorf("Expected speakers in dialogue chunk, got none: %+v", c)
			}
		}
	}
	if !foundDialogue {
		t.Errorf("Expected at least one dialogue chunk")
	}
}

package worker

import "testing"

func TestLadderForTrimsToSourceResolution(t *testing.T) {
	cases := []struct {
		name          string
		width, height int
		wantNames     []string
	}{
		{"1080p source gets the full ladder", 1920, 1080, []string{"1080p", "720p", "480p", "360p"}},
		{"720p source never gets upscaled to 1080p", 1280, 720, []string{"720p", "480p", "360p"}},
		{"480p source", 854, 480, []string{"480p", "360p"}},
		{"exactly 360p source", 640, 360, []string{"360p"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ladderFor(tc.width, tc.height)
			if len(got) != len(tc.wantNames) {
				t.Fatalf("ladderFor(%d,%d) = %d renditions, want %d (%v)", tc.width, tc.height, len(got), len(tc.wantNames), tc.wantNames)
			}
			for i, r := range got {
				if r.Name != tc.wantNames[i] {
					t.Errorf("rendition %d = %q, want %q", i, r.Name, tc.wantNames[i])
				}
				if r.Height > tc.height {
					t.Errorf("rendition %q (height %d) exceeds source height %d -- would upscale", r.Name, r.Height, tc.height)
				}
			}
		})
	}
}

func TestLadderForBelowLowestRungSynthesizesFallback(t *testing.T) {
	// A source shorter than even 360p should still produce one usable
	// rendition at its own size, not an empty ladder.
	got := ladderFor(426, 240)
	if len(got) != 1 {
		t.Fatalf("ladderFor(426,240) = %d renditions, want 1 fallback rendition; got %+v", len(got), got)
	}
	if got[0].Height != 240 {
		t.Errorf("fallback rendition height = %d, want 240 (source height, unmodified since already even)", got[0].Height)
	}
	if got[0].Width%2 != 0 || got[0].Height%2 != 0 {
		t.Errorf("fallback rendition %dx%d has an odd dimension, which breaks yuv420p encoding", got[0].Width, got[0].Height)
	}
}

func TestLadderForZeroResolutionReturnsNothing(t *testing.T) {
	if got := ladderFor(0, 0); got != nil {
		t.Errorf("ladderFor(0,0) = %+v, want nil", got)
	}
}

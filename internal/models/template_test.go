package models

import (
	"encoding/json"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func intPtr(i int) *int { return &i }

// minecraftGames mirrors a small games index with minecraft resolvable to its
// external id.
func minecraftGames() GamesIndex {
	return GamesIndex{
		"minecraft": {Name: "Minecraft", Slug: "minecraft", ExternalGameID: intPtr(38365)},
		"noid":      {Name: "No Id Game", Slug: "noid"},
	}
}

// T-TS1: both numeric and slug game_id must unmarshal without dropping the
// template.
func TestGameID_UnmarshalsStringOrNumber(t *testing.T) {
	numeric := `
name: Numeric
description: d
game_id: 38365
docker_image_name: itzg/minecraft-server
docker_image_tag: latest
`
	var tn Template
	if err := yaml.Unmarshal([]byte(numeric), &tn); err != nil {
		t.Fatalf("numeric game_id failed to unmarshal: %v", err)
	}
	if v, ok := tn.GameID.Value().(int64); !ok || v != 38365 {
		t.Fatalf("expected int64 38365, got %#v", tn.GameID.Value())
	}

	slug := `
name: Slug
description: d
game_id: minecraft
docker_image_name: itzg/minecraft-server
docker_image_tag: latest
`
	var ts Template
	if err := yaml.Unmarshal([]byte(slug), &ts); err != nil {
		t.Fatalf("slug game_id failed to unmarshal: %v", err)
	}
	if v, ok := ts.GameID.Value().(string); !ok || v != "minecraft" {
		t.Fatalf("expected string minecraft, got %#v", ts.GameID.Value())
	}
}

// T-TS5: numeric game_id passes through unchanged in v1/v2.
func TestToV2_NumericGameIDPassthrough(t *testing.T) {
	yml := `
name: Minecraft Vanilla
description: d
game_id: 38365
docker_image_name: itzg/minecraft-server
docker_image_tag: latest
resource_limit:
  memory: 2GiB
  cpu: 2
port_mapping:
  '25565/tcp': 25565
`
	var tmpl Template
	if err := yaml.Unmarshal([]byte(yml), &tmpl); err != nil {
		t.Fatal(err)
	}
	v2 := tmpl.ToV2(minecraftGames())
	if v2.GameID == nil || *v2.GameID != 38365 {
		t.Fatalf("expected game_id 38365, got %v", v2.GameID)
	}
	if v2.ResourceLimit == nil || v2.ResourceLimit.Memory != "2GiB" || v2.ResourceLimit.CPU != 2 {
		t.Fatalf("unexpected resource limit: %+v", v2.ResourceLimit)
	}
	if v2.PortMapping["25565/tcp"] != int64(25565) {
		t.Fatalf("expected port 25565 int64, got %#v", v2.PortMapping["25565/tcp"])
	}
}

// T-TS5: slug game_id resolves to numeric external id via the index.
func TestToV2_SlugGameIDResolves(t *testing.T) {
	yml := `
name: Slug
description: d
game_id: minecraft
docker_image_name: itzg/minecraft-server
docker_image_tag: latest
`
	var tmpl Template
	if err := yaml.Unmarshal([]byte(yml), &tmpl); err != nil {
		t.Fatal(err)
	}
	v2 := tmpl.ToV2(minecraftGames())
	if v2.GameID == nil || *v2.GameID != 38365 {
		t.Fatalf("expected resolved game_id 38365, got %v", v2.GameID)
	}
}

// T-TS5: slug with no external_game_id omits game_id gracefully.
func TestToV2_SlugWithoutExternalIDOmitted(t *testing.T) {
	yml := `
name: NoId
description: d
game_id: noid
docker_image_name: a/b
docker_image_tag: latest
`
	var tmpl Template
	if err := yaml.Unmarshal([]byte(yml), &tmpl); err != nil {
		t.Fatal(err)
	}
	v2 := tmpl.ToV2(minecraftGames())
	if v2.GameID != nil {
		t.Fatalf("expected game_id omitted, got %v", *v2.GameID)
	}
	out, _ := json.Marshal(v2)
	if got := string(out); contains(got, "game_id") {
		t.Fatalf("game_id should be omitted from JSON, got %s", got)
	}
}

// T-TS5: {{var}} in resource_limit is absent in v2 but present in v3.
func TestVarInResourceLimit_AbsentV2PresentV3(t *testing.T) {
	yml := `
name: Var
description: d
game_id: 38365
docker_image_name: a/b
docker_image_tag: latest
resource_limit:
  memory: '{{memory}}'
  cpu: '{{cpu}}'
port_mapping:
  '25565/tcp': '{{port}}'
  '25566/tcp': 25566
`
	var tmpl Template
	if err := yaml.Unmarshal([]byte(yml), &tmpl); err != nil {
		t.Fatal(err)
	}

	// v3 (raw) keeps the placeholders.
	v3, _ := json.Marshal(tmpl)
	if !contains(string(v3), "{{memory}}") || !contains(string(v3), "{{port}}") {
		t.Fatalf("v3 should keep placeholders, got %s", v3)
	}

	// v2 omits the {{var}} resource_limit fields entirely.
	v2 := tmpl.ToV2(minecraftGames())
	if v2.ResourceLimit != nil {
		t.Fatalf("expected resource_limit nil (both fields are vars), got %+v", v2.ResourceLimit)
	}
	// v2 omits the {{var}} port entry but keeps the numeric one.
	if _, ok := v2.PortMapping["25565/tcp"]; ok {
		t.Fatalf("expected {{var}} port omitted, got %v", v2.PortMapping)
	}
	if v2.PortMapping["25566/tcp"] != int64(25566) {
		t.Fatalf("expected numeric port retained, got %#v", v2.PortMapping["25566/tcp"])
	}
}

// T-TS5: partial {{var}} resource_limit keeps the non-var field.
func TestPartialVarResourceLimit(t *testing.T) {
	yml := `
name: Partial
description: d
game_id: 38365
docker_image_name: a/b
docker_image_tag: latest
resource_limit:
  memory: 2GiB
  cpu: '{{cpu}}'
`
	var tmpl Template
	if err := yaml.Unmarshal([]byte(yml), &tmpl); err != nil {
		t.Fatal(err)
	}
	v2 := tmpl.ToV2(minecraftGames())
	if v2.ResourceLimit == nil || v2.ResourceLimit.Memory != "2GiB" {
		t.Fatalf("expected memory retained, got %+v", v2.ResourceLimit)
	}
	if v2.ResourceLimit.CPU != 0 {
		t.Fatalf("expected cpu omitted (zero), got %v", v2.ResourceLimit.CPU)
	}
}

// T-TS5 regression: an unmigrated template produces v1/v2 output identical to
// the legacy shape (numeric game_id, typed resource_limit, default key in v1,
// default_value key in v2, annotations/host_mounts absent).
func TestUnmigratedTemplate_LegacyShape(t *testing.T) {
	yml := `
name: Minecraft Vanilla
description: Plain vanilla
variables:
  - name: Minecraft Version
    type: string
    placeholder: version
    default: '2'
game_id: 38365
docker_image_name: itzg/minecraft-server
docker_image_tag: latest
environment_variables:
  EULA: 'true'
port_mapping:
  '25565/tcp': 25565
file_mounts:
  - /data
resource_limit:
  memory: 2GiB
  cpu: 2
tags:
  - vanilla
`
	var tmpl Template
	if err := yaml.Unmarshal([]byte(yml), &tmpl); err != nil {
		t.Fatal(err)
	}

	v1, _ := json.Marshal(tmpl.ToV1(minecraftGames()))
	v1s := string(v1)
	for _, want := range []string{`"game_id":38365`, `"default":"2"`, `"memory":"2GiB"`, `"cpu":2`, `"25565/tcp":25565`} {
		if !contains(v1s, want) {
			t.Fatalf("v1 missing %q in %s", want, v1s)
		}
	}
	for _, bad := range []string{"annotations", "host_mounts", "default_value"} {
		if contains(v1s, bad) {
			t.Fatalf("v1 should not contain %q in %s", bad, v1s)
		}
	}

	v2, _ := json.Marshal(tmpl.ToV2(minecraftGames()))
	v2s := string(v2)
	for _, want := range []string{`"game_id":38365`, `"default_value":"2"`, `"memory":"2GiB"`, `"cpu":2`} {
		if !contains(v2s, want) {
			t.Fatalf("v2 missing %q in %s", want, v2s)
		}
	}
	for _, bad := range []string{"annotations", "host_mounts"} {
		if contains(v2s, bad) {
			t.Fatalf("v2 should not contain %q in %s", bad, v2s)
		}
	}
}

// T-TS2/T-TS4: v3 raw output includes annotations + host_mounts with snake_case
// nested fields.
func TestV3_NewFieldsPresent(t *testing.T) {
	yml := `
name: Router
description: d
game_id: minecraft
docker_image_name: a/b
docker_image_tag: latest
annotations:
  traefik.enable: 'true'
host_mounts:
  - host_path: /var/run/docker.sock
    container_path: /var/run/docker.sock
    read_only: false
`
	var tmpl Template
	if err := yaml.Unmarshal([]byte(yml), &tmpl); err != nil {
		t.Fatal(err)
	}
	out, _ := json.Marshal(tmpl)
	s := string(out)
	for _, want := range []string{`"annotations"`, `"traefik.enable":"true"`, `"host_path":"/var/run/docker.sock"`, `"container_path"`, `"game_id":"minecraft"`} {
		if !contains(s, want) {
			t.Fatalf("v3 missing %q in %s", want, s)
		}
	}
}

// T-TS2: an omitted read_only defaults to true on the v3 wire.
func TestHostMount_ReadOnlyDefaultsTrue(t *testing.T) {
	yml := `
name: Router
description: d
game_id: minecraft
docker_image_name: a/b
docker_image_tag: latest
host_mounts:
  - host_path: /data
    container_path: /data
`
	var tmpl Template
	if err := yaml.Unmarshal([]byte(yml), &tmpl); err != nil {
		t.Fatal(err)
	}
	if len(tmpl.HostMounts) != 1 || tmpl.HostMounts[0].ReadOnly != nil {
		t.Fatalf("expected nil read_only in model, got %+v", tmpl.HostMounts)
	}
	out, _ := json.Marshal(tmpl)
	if !contains(string(out), `"read_only":true`) {
		t.Fatalf("expected read_only true on wire, got %s", out)
	}
}

// Regression: a resource_limit with only one field set must not emit null for
// the absent field in the v3 wire output.
func TestPartialResourceLimit_V3NoNull(t *testing.T) {
	yml := `
name: Partial
description: d
game_id: 38365
docker_image_name: a/b
docker_image_tag: latest
resource_limit:
  memory: 2GiB
`
	var tmpl Template
	if err := yaml.Unmarshal([]byte(yml), &tmpl); err != nil {
		t.Fatal(err)
	}
	out, _ := json.Marshal(tmpl)
	s := string(out)
	if contains(s, "null") {
		t.Fatalf("partial resource_limit emits null: %s", s)
	}
	if !contains(s, `"memory":"2GiB"`) {
		t.Fatalf("memory missing from v3 output: %s", s)
	}
	if contains(s, `"cpu"`) {
		t.Fatalf("absent cpu should not appear in v3 output: %s", s)
	}
}

func TestGame_JSONShape(t *testing.T) {
	g := Game{Name: "Minecraft", LogoURL: "https://cdn/logo.png", HeroURL: "https://cdn/hero.png", ExternalGameID: intPtr(38365), Slug: "minecraft"}
	out, _ := json.Marshal(g)
	s := string(out)
	for _, want := range []string{`"name":"Minecraft"`, `"logo_url":"https://cdn/logo.png"`, `"hero_url":"https://cdn/hero.png"`, `"external_game_id":38365`, `"slug":"minecraft"`} {
		if !contains(s, want) {
			t.Fatalf("game JSON missing %q in %s", want, s)
		}
	}
}

// v1/v2 consumers store the description in a varchar(255) column, so a
// short description must pass through untouched in both versions.
func TestToV1V2_ShortDescriptionPassthrough(t *testing.T) {
	desc := strings.Repeat("a", 255)
	yml := `
name: Short
description: ` + desc + `
game_id: 38365
docker_image_name: a/b
docker_image_tag: latest
`
	var tmpl Template
	if err := yaml.Unmarshal([]byte(yml), &tmpl); err != nil {
		t.Fatal(err)
	}
	if got := tmpl.ToV1(minecraftGames()).Description; got != desc {
		t.Fatalf("v1 description changed: len %d", len([]rune(got)))
	}
	if got := tmpl.ToV2(minecraftGames()).Description; got != desc {
		t.Fatalf("v2 description changed: len %d", len([]rune(got)))
	}
}

// A description over 255 chars is truncated to exactly 255 runes ending in
// "..." in both v1 and v2.
func TestToV1V2_LongDescriptionTruncated(t *testing.T) {
	yml := `
name: Long
description: ` + strings.Repeat("a", 358) + `
game_id: 38365
docker_image_name: a/b
docker_image_tag: latest
`
	var tmpl Template
	if err := yaml.Unmarshal([]byte(yml), &tmpl); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		got  string
	}{
		{"v1", tmpl.ToV1(minecraftGames()).Description},
		{"v2", tmpl.ToV2(minecraftGames()).Description},
	} {
		if n := len([]rune(tc.got)); n != 255 {
			t.Fatalf("%s: expected 255 runes, got %d", tc.name, n)
		}
		if !strings.HasSuffix(tc.got, "...") {
			t.Fatalf("%s: expected trailing ..., got %q", tc.name, tc.got)
		}
	}
}

// The 255-rune cap counts runes, not bytes: 300 multi-byte 'ä' runes truncate
// to exactly 255 runes (252 'ä' + "...").
func TestToV1V2_MultiByteDescriptionCountedInRunes(t *testing.T) {
	yml := `
name: Umlaut
description: ` + strings.Repeat("ä", 300) + `
game_id: 38365
docker_image_name: a/b
docker_image_tag: latest
`
	var tmpl Template
	if err := yaml.Unmarshal([]byte(yml), &tmpl); err != nil {
		t.Fatal(err)
	}
	got := tmpl.ToV2(minecraftGames()).Description
	if n := len([]rune(got)); n != 255 {
		t.Fatalf("expected 255 runes, got %d (bytes %d)", n, len(got))
	}
	if want := strings.Repeat("ä", 252) + "..."; got != want {
		t.Fatalf("unexpected truncation: %q", got)
	}
}

func contains(haystack, needle string) bool {
	return strings.Contains(haystack, needle)
}

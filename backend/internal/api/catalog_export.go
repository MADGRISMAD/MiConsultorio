package api

import (
	"slices"
	"sort"
)

// Read-only views of the embedded clinical catalogs, for cmd/exportcatalogs (the clinical review package).
// They return copies: nothing here changes a catalog.

type (
	// CatalogMed is one medicine of the receta catalog (human or veterinary).
	CatalogMed = catMed
	// CatalogICD is one CIE-10 entry.
	CatalogICD = icdEntry
	// CatalogVaccine is one suggested vaccine or deworming of a scheme.
	CatalogVaccine = vaccineSuggestion
)

// CatalogMedications returns every medicine of the catalog, in catalog order.
func CatalogMedications() []CatalogMed { return slices.Clone(catalogMeds) }

// CatalogICD10 returns the partial CIE-10 list, sorted by code.
func CatalogICD10() []CatalogICD { return slices.Clone(icd10) }

// CatalogVaccineSchemes returns the suggested vaccination and deworming schemes by species ("person" for people).
func CatalogVaccineSchemes() (names []string, schemes map[string][]CatalogVaccine) {
	schemes = map[string][]CatalogVaccine{}
	for k, v := range vaccineCatalog {
		names = append(names, k)
		schemes[k] = slices.Clone(v)
	}
	sort.Strings(names)
	return names, schemes
}

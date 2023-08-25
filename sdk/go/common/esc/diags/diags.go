// Copyright 2022, Pulumi Corporation.  All rights reserved.

package diags

import (
	"fmt"
	"sort"
	"strings"
)

// A formatter for when a field or property is used that does not exist.
type NonExistantFieldFormatter struct {
	ParentLabel         string
	Fields              []string
	MaxElements         int
	FieldsAreProperties bool
}

func (e NonExistantFieldFormatter) fieldsName() string {
	if e.FieldsAreProperties {
		return "properties"
	}
	return "fields"
}

// Get a single line message.
func (e NonExistantFieldFormatter) Message(field, fieldLabel string) string {
	return fmt.Sprintf("%s %s", e.messageHeader(fieldLabel), e.messageBody(field))
}

// A message broken up into a top level and detail line
func (e NonExistantFieldFormatter) MessageWithDetail(field, fieldLabel string) (string, string) {
	return e.messageHeader(fieldLabel), e.messageBody(field)
}

func (e NonExistantFieldFormatter) messageHeader(fieldLabel string) string {
	return fmt.Sprintf("%s does not exist on %s", fieldLabel, e.ParentLabel)
}

func (e NonExistantFieldFormatter) messageBody(field string) string {
	existing := sortByEditDistance(e.Fields, field)
	if len(existing) == 0 {
		return fmt.Sprintf("%s has no %s", e.ParentLabel, e.fieldsName())
	}
	list := strings.Join(existing, ", ")
	if len(existing) > e.MaxElements && e.MaxElements != 0 {
		list = fmt.Sprintf("%s and %d others", strings.Join(existing[:5], ", "), len(existing)-e.MaxElements)
	}
	return fmt.Sprintf("Existing %s are: %s", e.fieldsName(), list)
}

// A formatter for missing fields that may be valid in other places.
type InvalidFieldBagFormatter struct {
	ParentLabel string
	Bags        []TypeBag
	// The maximum number of fields to return. `MaxListed` <= 0 implies return all fields.
	MaxListed int
	// The degree of (edit) distance between the field typed and suggested.
	DistanceLimit int
}

type TypeBag struct {
	Name       string
	Properties []string
}

func (e InvalidFieldBagFormatter) BagList(field string) []BagOrdering {
	bags := suggestBag(field, e.Bags)

	if e.DistanceLimit != 0 {
		b := []BagOrdering{}
		for _, bag := range bags {
			if bag.rank > e.DistanceLimit {
				break
			}
			b = append(b, bag)
		}
		bags = b
	}

	if e.MaxListed > 0 {
		b := []BagOrdering{}
		for i := 0; i < e.MaxListed && i < len(bags); i++ {
			b = append(b, bags[i])
		}
		bags = b
	}
	return bags
}

// The list of bags that exactly match `field`.
func (e InvalidFieldBagFormatter) ExactMatching(field string) []string {
	m := map[string]struct{}{}
	for _, bag := range e.BagList(field) {
		if bag.rank == 0 {
			for _, name := range bag.BagName {
				m[name] = struct{}{}
			}
		}
	}
	names := make([]string, 0, len(m))
	for bag := range m {
		names = append(names, bag)
	}
	return names
}

func (e InvalidFieldBagFormatter) MessageWithDetail(field string) (string, string) {
	bags := e.BagList(field)
	summary := fmt.Sprintf("%s has invalid key '%s'", e.ParentLabel, field)

	// No matches, so say nothing
	if len(bags) == 0 {
		return summary, ""
	}

	// We have an exact match, so only show exact matches.
	if bags[0].rank == 0 {
		names := e.ExactMatching(field)
		detail := fmt.Sprintf("'%s' exists under %s", field, AndList(names))
		return summary, detail
	}

	// We have matches but none are exact
	detail := "Did you mean "
	addBag := func(bag BagOrdering) {
		if len(bag.BagName) == 1 {
			detail += fmt.Sprintf("'%s' under '%s'", bag.Property, bag.BagName[0])
		} else {
			names := OrList{}
			for _, name := range bag.BagName {
				names = append(names, fmt.Sprintf("'%s'", name))
			}
			detail += fmt.Sprintf("'%s' under keys %s", bag.Property, names)
		}
	}
	if len(bags) == 1 {
		addBag(bags[0])
		detail += "?"
	} else {
		detail += "one of the following:\n"
		for _, bag := range bags {
			detail += "  * "
			addBag(bag)
			detail += "\n"
		}
	}
	return summary, detail
}

type BagOrdering struct {
	// Which bags `Property` belongs in.
	BagName []string
	// The name of the property
	Property string

	// Computed edit distance from the `field` that this was created from.
	rank int
}

// Infer which of several property bags a value is most likely to be in.
func suggestBag(field string, bags []TypeBag) []BagOrdering {
	places := map[string]BagOrdering{}
	for _, bag := range bags {
		for _, prop := range bag.Properties {
			existing, ok := places[prop]
			if !ok {
				existing = BagOrdering{
					Property: prop,
				}
			}
			existing.BagName = append(existing.BagName, bag.Name)
			places[prop] = existing
		}
	}
	orderedPlaces := make([]BagOrdering, 0, len(places))
	for _, p := range places {
		p.rank = editDistance(field, p.Property)
		orderedPlaces = append(orderedPlaces, p)
	}
	sort.Slice(orderedPlaces, func(i, j int) bool {
		return orderedPlaces[i].rank < orderedPlaces[j].rank
	})
	return orderedPlaces
}

// Copyright 2018 The mkcert Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type validityDuration struct {
	years   int
	months  int
	days    int
	timeDur time.Duration
}

func (v validityDuration) apply(t time.Time) time.Time {
	return t.AddDate(v.years, v.months, v.days).Add(v.timeDur)
}

var (
	validityPartPattern = regexp.MustCompile(`(?i)^(\d+)\s*([a-z]+)$`)
	validityTokenizer   = regexp.MustCompile(`(?i)\d+\s*[a-z]+`)
)

func parseValidity(s string) (validityDuration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return validityDuration{}, fmt.Errorf("validity duration cannot be empty")
	}

	// Verify that the string is completely covered by valid component tokens
	matches := validityTokenizer.FindAllStringIndex(s, -1)
	if len(matches) == 0 {
		return validityDuration{}, fmt.Errorf("invalid validity format %q (examples: 10y, 2y3m, 365d, 30d, 24h)", s)
	}

	// Check for extraneous characters between or around matches
	lastIdx := 0
	for _, idx := range matches {
		gap := strings.TrimSpace(s[lastIdx:idx[0]])
		if gap != "" {
			return validityDuration{}, fmt.Errorf("invalid characters %q in validity %q", gap, s)
		}
		lastIdx = idx[1]
	}
	remaining := strings.TrimSpace(s[lastIdx:])
	if remaining != "" {
		return validityDuration{}, fmt.Errorf("invalid characters %q in validity %q", remaining, s)
	}

	var res validityDuration
	for _, idx := range matches {
		token := s[idx[0]:idx[1]]
		parts := validityPartPattern.FindStringSubmatch(token)
		if len(parts) != 3 {
			return validityDuration{}, fmt.Errorf("invalid validity component %q", token)
		}

		val, err := strconv.Atoi(parts[1])
		if err != nil || val < 0 {
			return validityDuration{}, fmt.Errorf("invalid number %q in validity %q", parts[1], s)
		}

		unit := strings.ToLower(parts[2])
		switch unit {
		case "y", "yr", "yrs", "year", "years":
			res.years += val
		case "mo", "mon", "mons", "month", "months", "m":
			res.months += val
		case "d", "day", "days":
			res.days += val
		case "h", "hr", "hrs", "hour", "hours":
			res.timeDur += time.Duration(val) * time.Hour
		case "min", "mins", "minute", "minutes":
			res.timeDur += time.Duration(val) * time.Minute
		case "s", "sec", "secs", "second", "seconds":
			res.timeDur += time.Duration(val) * time.Second
		default:
			return validityDuration{}, fmt.Errorf("unknown unit %q in validity %q (supported units: y, mo/m, d, h, min, s)", unit, s)
		}
	}

	if res.years == 0 && res.months == 0 && res.days == 0 && res.timeDur == 0 {
		return validityDuration{}, fmt.Errorf("validity must be greater than zero")
	}

	return res, nil
}

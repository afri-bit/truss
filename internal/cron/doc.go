// Package cron parses cron expressions and computes the times they match.
//
// It supports the classic five fields (minute, hour, day of month, month, day
// of week) with lists, ranges, steps and three-letter names, plus the
// shortcuts @hourly, @daily, @midnight, @weekly, @monthly and @yearly.
// It does not run anything; the scheduler decides what a matching time means.
package cron

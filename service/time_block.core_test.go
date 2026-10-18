package service

import (
	"testing"
	"time"

	"github.com/Nina-99/yotuvia-api/models"
	"github.com/stretchr/testify/assert"
)

// Time-block helpers feeding the slot kernel / write guard. The semantics worth
// pinning: a centre-wide block (PersonnelID nil) applies to everyone and to a
// centre-wide search; a professional's own block applies only to them (a
// colleague's break must not hide the centre).

func uintPtr(v uint) *uint { return &v }

func TestBlockBusyReservations_Scope(t *testing.T) {
	date := time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)
	blocks := []models.TimeBlock{
		{BeautyCenterID: 1, Date: date, StartTime: "12:00", EndTime: "13:00"},          // centre-wide
		{BeautyCenterID: 1, Date: date, StartTime: "15:00", EndTime: "16:00", PersonnelID: uintPtr(7)}, // pro 7
	}

	t.Run("centre-wide search only sees centre-wide blocks", func(t *testing.T) {
		got := BlockBusyReservations(blocks, 0)
		assert.Len(t, got, 1)
		assert.Equal(t, "12:00", got[0].StartTime)
	})

	t.Run("a professional sees centre-wide plus their own", func(t *testing.T) {
		got := BlockBusyReservations(blocks, 7)
		assert.Len(t, got, 2)
	})

	t.Run("another professional sees only the centre-wide block", func(t *testing.T) {
		got := BlockBusyReservations(blocks, 9)
		assert.Len(t, got, 1)
		assert.Equal(t, "12:00", got[0].StartTime)
	})
}

func TestWindowOverlapsBlocks(t *testing.T) {
	block := models.TimeBlock{BeautyCenterID: 1, StartTime: "12:00", EndTime: "13:00"}

	assert.True(t, WindowOverlapsBlocks([]models.TimeBlock{block}, 0, 12*60, 13*60), "exact overlap")
	assert.True(t, WindowOverlapsBlocks([]models.TimeBlock{block}, 0, 12*60+30, 13*60+30), "partial overlap")
	assert.False(t, WindowOverlapsBlocks([]models.TimeBlock{block}, 0, 13*60, 14*60), "touching end does not overlap")
	assert.False(t, WindowOverlapsBlocks([]models.TimeBlock{block}, 0, 11*60, 12*60), "touching start does not overlap")

	proBlock := models.TimeBlock{PersonnelID: uintPtr(7), StartTime: "12:00", EndTime: "13:00"}
	assert.True(t, WindowOverlapsBlocks([]models.TimeBlock{proBlock}, 7, 12*60, 13*60), "the pro is blocked")
	assert.False(t, WindowOverlapsBlocks([]models.TimeBlock{proBlock}, 9, 12*60, 13*60), "another pro is not")
	assert.False(t, WindowOverlapsBlocks([]models.TimeBlock{proBlock}, 0, 12*60, 13*60), "a centre-wide search ignores a single pro's block")
}

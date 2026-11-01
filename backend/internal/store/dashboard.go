package store

import (
	"context"
	"time"
)

// DashboardSummary is the counter block at the top of the workbench.
type DashboardSummary struct {
	Users            int64 `json:"users"`
	EnabledUsers     int64 `json:"enabledUsers"`
	OnlineSessions   int64 `json:"onlineSessions"`
	Departments      int64 `json:"departments"`
	Roles            int64 `json:"roles"`
	Files            int64 `json:"files"`
	StorageBytes     int64 `json:"storageBytes"`
	UnreadNotices    int64 `json:"unreadNotices"`
	TodayLogins      int64 `json:"todayLogins"`
	TodayFailures    int64 `json:"todayFailures"`
	UnresolvedErrors int64 `json:"unresolvedErrors"`
}

// DashboardTrend is one day of the login trend chart.
type DashboardTrend struct {
	Date    string `json:"date"`
	Success int64  `json:"success"`
	Failure int64  `json:"failure"`
}

// Dashboard loads the summary counters and a fixed seven-day login trend. Days without logins are filled with zero so the chart never has gaps.
func (r *AuthRepository) Dashboard(ctx context.Context, userID uint64) (DashboardSummary, []DashboardTrend, error) {
	row, err := r.queries.GetDashboardSummary(ctx, userID)
	if err != nil {
		return DashboardSummary{}, nil, err
	}
	summary := DashboardSummary{
		Users: row.Users, EnabledUsers: row.EnabledUsers, OnlineSessions: row.OnlineSessions,
		Departments: row.Departments, Roles: row.Roles, Files: row.Files,
		StorageBytes: int64(row.StorageBytes), UnreadNotices: row.UnreadNotices,
		TodayLogins: row.TodayLogins, TodayFailures: row.TodayFailures, UnresolvedErrors: row.UnresolvedErrors,
	}
	today := time.Now()
	trendMap := make(map[string]*DashboardTrend, 7)
	trends := make([]DashboardTrend, 7)
	for index, offset := 0, 6; offset >= 0; index, offset = index+1, offset-1 {
		date := today.AddDate(0, 0, -offset).Format("2006-01-02")
		trends[index] = DashboardTrend{Date: date}
		trendMap[date] = &trends[index]
	}
	rows, err := r.queries.ListLoginTrendLast7Days(ctx)
	if err != nil {
		return DashboardSummary{}, nil, err
	}
	for _, item := range rows {
		if trend := trendMap[item.Date]; trend != nil {
			if smallIntBool(item.Success) {
				trend.Success = item.Count
			} else {
				trend.Failure = item.Count
			}
		}
	}
	return summary, trends, nil
}

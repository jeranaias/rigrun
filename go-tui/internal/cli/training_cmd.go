// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

// training_cmd.go - CLI commands for AT-2/AT-3 training management in rigrun.
//
// CLI: Comprehensive help and examples for all commands
//
// Implements NIST 800-53 AT-2 (Security Awareness Training) and
// AT-3 (Role-Based Training) controls for DoD IL5 compliance.
//
// Command: training [subcommand]
// Short:   Training management (IL5 AT-2, AT-3)
// Aliases: train
//
// Subcommands:
//   status (default)    Show current user's training status
//   required [role]     List required training for role
//   complete <id> <score>  Record training completion
//   history             Show training history
//   expiring [days]     Show training expiring in N days
//   report              Generate compliance report
//   courses             List all available courses
//
// Examples:
//   rigrun training                       Show status (default)
//   rigrun training status                Show training status
//   rigrun training status --json         Status in JSON format
//   rigrun training required              Required for current role
//   rigrun training required admin        Required for admin role
//   rigrun training complete SEC-101 95   Record completion
//   rigrun training history               View history
//   rigrun training expiring 30           Training expiring in 30 days
//   rigrun training report                Compliance report
//   rigrun training courses               List courses
//
// Required Training (IL5):
//   - Security Awareness (Annual)
//   - Insider Threat (Annual)
//   - Classified Handling (as required)
//   - Role-specific training
//
// Flags:
//   --json              Output in JSON format
//
// Copyright (c) 2024-2025 Jesse Morgan / Morgan Forge
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/jeranaias/rigrun-tui/internal/security"
	"github.com/jeranaias/rigrun-tui/internal/util"
)

// =============================================================================
// TRAINING COMMAND STYLES
// =============================================================================

var (
	trainingTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("39")). // Cyan
				MarginBottom(1)

	trainingSectionStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("255")). // White
				MarginTop(1)

	trainingLabelStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("245")). // Light gray
				Width(20)

	trainingValueStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("255")) // White

	trainingGreenStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("82")) // Green

	trainingRedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("196")) // Red

	trainingYellowStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("220")) // Yellow

	trainingDimStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("242")) // Dim

	trainingSuccessStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("82")).
				Bold(true)

	trainingErrorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("196")).
				Bold(true)

	trainingOrangeStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("208")) // Orange
)

// =============================================================================
// TRAINING ARGUMENTS
// =============================================================================

// TrainingArgs holds parsed training command arguments.
type TrainingArgs struct {
	Subcommand string
	CourseID   string
	Score      float64
	Days       int
	Role       string
	JSON       bool
}

// parseTrainingCmdArgs parses training command specific arguments.
func parseTrainingCmdArgs(args *Args, remaining []string) TrainingArgs {
	trainingArgs := TrainingArgs{
		JSON: args.JSON,
		Days: 30, // Default to 30 days for expiring
		Role: "user", // Default role
	}

	if len(remaining) > 0 {
		trainingArgs.Subcommand = remaining[0]
		remaining = remaining[1:]
	}

	// Parse subcommand-specific args
	switch trainingArgs.Subcommand {
	case "complete":
		if len(remaining) > 0 {
			trainingArgs.CourseID = remaining[0]
		}
		if len(remaining) > 1 {
			if score, err := strconv.ParseFloat(remaining[1], 64); err == nil {
				trainingArgs.Score = score
			}
		}

	case "expiring":
		if len(remaining) > 0 {
			if days, err := strconv.Atoi(remaining[0]); err == nil {
				trainingArgs.Days = days
			}
		}

	case "required":
		if len(remaining) > 0 {
			trainingArgs.Role = remaining[0]
		}
	}

	// Parse flags
	for i := 0; i < len(remaining); i++ {
		arg := remaining[i]

		switch arg {
		case "--role", "-r":
			if i+1 < len(remaining) {
				i++
				trainingArgs.Role = remaining[i]
			}
		case "--days", "-d":
			if i+1 < len(remaining) {
				i++
				if days, err := strconv.Atoi(remaining[i]); err == nil {
					trainingArgs.Days = days
				}
			}
		case "--json":
			trainingArgs.JSON = true
		default:
			if strings.HasPrefix(arg, "--role=") {
				trainingArgs.Role = strings.TrimPrefix(arg, "--role=")
			} else if strings.HasPrefix(arg, "--days=") {
				if days, err := strconv.Atoi(strings.TrimPrefix(arg, "--days=")); err == nil {
					trainingArgs.Days = days
				}
			}
		}
	}

	return trainingArgs
}

// =============================================================================
// HANDLE TRAINING
// =============================================================================

// HandleTraining handles the "training" command with various subcommands.
// Implements AT-2/AT-3 management commands.
func HandleTraining(args Args) error {
	trainingArgs := parseTrainingCmdArgs(&args, args.Raw)

	switch trainingArgs.Subcommand {
	case "", "status":
		return handleTrainingStatus(trainingArgs)
	case "required":
		return handleTrainingRequired(trainingArgs)
	case "complete":
		return handleTrainingComplete(trainingArgs)
	case "history":
		return handleTrainingHistory(trainingArgs)
	case "expiring":
		return handleTrainingExpiring(trainingArgs)
	case "report":
		return handleTrainingReport(trainingArgs)
	case "courses":
		return handleTrainingCourses(trainingArgs)
	default:
		return fmt.Errorf("unknown training subcommand: %s\n\nUsage:\n"+
			"  rigrun training status              Show current user's training status\n"+
			"  rigrun training required [role]     List required training for role\n"+
			"  rigrun training complete [courseID] [score]  Record training completion\n"+
			"  rigrun training history             Show training history\n"+
			"  rigrun training expiring [days]     Show training expiring soon\n"+
			"  rigrun training report              Generate compliance report\n"+
			"  rigrun training courses             List all available courses", trainingArgs.Subcommand)
	}
}

// =============================================================================
// TRAINING STATUS
// =============================================================================

// handleTrainingStatus shows the current user's training status.
func handleTrainingStatus(args TrainingArgs) error {
	trainingMgr := security.GlobalTrainingManager()
	authMgr := security.GlobalAuthManager()

	// Get current user ID
	sessions := authMgr.ListSessions()
	userID := "default_user"
	if len(sessions) > 0 && sessions[0].IsValid() {
		userID = sessions[0].UserID
	}

	// Parse role
	role := security.UserRole(args.Role)
	status := trainingMgr.GetTrainingStatus(userID, role)

	if args.JSON {
		data, err := json.MarshalIndent(status, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	// Display human-readable status
	separator := strings.Repeat("=", 60)
	fmt.Println()
	fmt.Println(trainingTitleStyle.Render("AT-2/AT-3 Training Status"))
	fmt.Println(trainingDimStyle.Render(separator))
	fmt.Println()

	// User info
	fmt.Println(trainingSectionStyle.Render("User Information"))
	fmt.Printf("  %s%s\n", trainingLabelStyle.Render("User ID:"), trainingValueStyle.Render(status.UserID))
	fmt.Printf("  %s%s\n", trainingLabelStyle.Render("Role:"), trainingValueStyle.Render(string(status.Role)))
	fmt.Println()

	// Training summary
	fmt.Println(trainingSectionStyle.Render("Training Summary"))
	fmt.Printf("  %s%s\n", trainingLabelStyle.Render("Required Courses:"), trainingValueStyle.Render(fmt.Sprintf("%d", status.RequiredCourses)))
	fmt.Printf("  %s%s\n", trainingLabelStyle.Render("Completed:"), trainingGreenStyle.Render(fmt.Sprintf("%d", status.CompletedCourses)))
	fmt.Printf("  %s%s\n", trainingLabelStyle.Render("Pending:"), trainingYellowStyle.Render(fmt.Sprintf("%d", status.PendingCourses)))

	if status.ExpiringCourses > 0 {
		fmt.Printf("  %s%s\n", trainingLabelStyle.Render("Expiring Soon:"), trainingOrangeStyle.Render(fmt.Sprintf("%d", status.ExpiringCourses)))
	}

	if status.ExpiredCourses > 0 {
		fmt.Printf("  %s%s\n", trainingLabelStyle.Render("Expired:"), trainingRedStyle.Render(fmt.Sprintf("%d", status.ExpiredCourses)))
	}

	if status.LastCompleted != nil {
		fmt.Printf("  %s%s\n", trainingLabelStyle.Render("Last Completed:"), trainingDimStyle.Render(status.LastCompleted.Format("2006-01-02")))
	}
	fmt.Println()

	// Compliance status
	fmt.Println(trainingSectionStyle.Render("NIST 800-53 AT-2/AT-3 Compliance"))
	complianceStr := trainingRedStyle.Render("NON-COMPLIANT")
	if status.IsCurrent {
		complianceStr = trainingGreenStyle.Render("COMPLIANT")
	}
	fmt.Printf("  %s%s\n", trainingLabelStyle.Render("Status:"), complianceStr)

	if !status.IsCurrent {
		fmt.Println()
		if status.PendingCourses > 0 {
			fmt.Println(trainingYellowStyle.Render(fmt.Sprintf("  %d course(s) pending completion", status.PendingCourses)))
		}
		if status.ExpiredCourses > 0 {
			fmt.Println(trainingRedStyle.Render(fmt.Sprintf("  %d course(s) expired - renewal required", status.ExpiredCourses)))
		}
		fmt.Println()
		fmt.Println(trainingDimStyle.Render("  Run 'rigrun training required' to see required courses"))
	}

	fmt.Println()
	return nil
}

// =============================================================================
// TRAINING REQUIRED
// =============================================================================

// handleTrainingRequired lists required training for a role.
func handleTrainingRequired(args TrainingArgs) error {
	trainingMgr := security.GlobalTrainingManager()
	role := security.UserRole(args.Role)
	courses := trainingMgr.GetRequiredTraining(role)

	if args.JSON {
		data, err := json.MarshalIndent(map[string]interface{}{
			"role":    role,
			"courses": courses,
			"count":   len(courses),
		}, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	fmt.Println()
	fmt.Println(trainingTitleStyle.Render(fmt.Sprintf("Required Training for Role: %s", role)))
	fmt.Println(trainingDimStyle.Render(strings.Repeat("=", 60)))
	fmt.Println()

	if len(courses) == 0 {
		fmt.Println(trainingDimStyle.Render("  No required training courses for this role."))
		fmt.Println()
		return nil
	}

	for _, course := range courses {
		fmt.Println(trainingSectionStyle.Render(course.Name))
		fmt.Printf("  %s%s\n", trainingLabelStyle.Render("Course ID:"), trainingValueStyle.Render(course.ID))
		fmt.Printf("  %s%s\n", trainingLabelStyle.Render("Type:"), trainingValueStyle.Render(string(course.Type)))
		fmt.Printf("  %s%s\n", trainingLabelStyle.Render("Duration:"), trainingValueStyle.Render(formatDurationShort(course.Duration)))
		fmt.Printf("  %s%s\n", trainingLabelStyle.Render("Passing Score:"), trainingValueStyle.Render(fmt.Sprintf("%.0f%%", course.PassingScore)))
		fmt.Printf("  %s%s\n", trainingLabelStyle.Render("Valid For:"), trainingValueStyle.Render(formatDurationShort(course.ExpirationPeriod)))
		fmt.Println()
		fmt.Println(trainingDimStyle.Render("  " + course.Description))
		fmt.Println()
	}

	fmt.Printf("  Total: %d required course(s)\n", len(courses))
	fmt.Println()

	return nil
}

// =============================================================================
// TRAINING COMPLETE
// =============================================================================

// handleTrainingComplete records a training completion.
func handleTrainingComplete(args TrainingArgs) error {
	if args.CourseID == "" {
		return fmt.Errorf("course ID required\n\nUsage: rigrun training complete [courseID] [score]")
	}

	if args.Score < 0 || args.Score > 100 {
		return fmt.Errorf("score must be between 0 and 100")
	}

	trainingMgr := security.GlobalTrainingManager()
	authMgr := security.GlobalAuthManager()

	// Get current user ID
	sessions := authMgr.ListSessions()
	userID := "default_user"
	if len(sessions) > 0 && sessions[0].IsValid() {
		userID = sessions[0].UserID
	}

	// Record completion
	err := trainingMgr.RecordCompletion(userID, args.CourseID, args.Score)

	// Get course info for display
	course, courseErr := trainingMgr.GetCourse(args.CourseID)
	if courseErr != nil {
		return courseErr
	}

	if args.JSON {
		result := map[string]interface{}{
			"user_id":   userID,
			"course_id": args.CourseID,
			"score":     args.Score,
			"passed":    err == nil,
		}
		if err != nil {
			result["error"] = err.Error()
		}
		data, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	fmt.Println()
	if err != nil {
		fmt.Printf("%s Training completion recorded but not passed\n", trainingYellowStyle.Render("[WARN]"))
		fmt.Printf("  Course:  %s\n", course.Name)
		fmt.Printf("  Score:   %.1f%% (required: %.0f%%)\n", args.Score, course.PassingScore)
		fmt.Printf("  Status:  %s\n", trainingRedStyle.Render("FAILED"))
		fmt.Println()
		fmt.Println(trainingYellowStyle.Render("  Retake required to achieve passing score"))
		fmt.Println()
		return nil
	}

	fmt.Printf("%s Training completed successfully\n", trainingSuccessStyle.Render("[OK]"))
	fmt.Printf("  Course:  %s\n", course.Name)
	fmt.Printf("  Score:   %.1f%%\n", args.Score)
	fmt.Printf("  Status:  %s\n", trainingGreenStyle.Render("PASSED"))
	fmt.Printf("  Expires: %s\n", time.Now().Add(course.ExpirationPeriod).Format("2006-01-02"))
	fmt.Println()

	return nil
}

// =============================================================================
// TRAINING HISTORY
// =============================================================================

// handleTrainingHistory shows a user's training history.
func handleTrainingHistory(args TrainingArgs) error {
	trainingMgr := security.GlobalTrainingManager()
	authMgr := security.GlobalAuthManager()

	// Get current user ID
	sessions := authMgr.ListSessions()
	userID := "default_user"
	if len(sessions) > 0 && sessions[0].IsValid() {
		userID = sessions[0].UserID
	}

	history := trainingMgr.GetTrainingHistory(userID)

	if args.JSON {
		data, err := json.MarshalIndent(map[string]interface{}{
			"user_id": userID,
			"history": history,
			"count":   len(history),
		}, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	fmt.Println()
	fmt.Println(trainingTitleStyle.Render("Training History"))
	fmt.Println(trainingDimStyle.Render(strings.Repeat("=", 80)))
	fmt.Println()

	if len(history) == 0 {
		fmt.Println(trainingDimStyle.Render("  No training history found."))
		fmt.Println()
		return nil
	}

	// Table header
	fmt.Printf("  %-25s %-12s %-12s %-10s %-10s\n", "Course", "Completed", "Expires", "Score", "Status")
	fmt.Println(trainingDimStyle.Render("  " + strings.Repeat("-", 75)))

	for _, record := range history {
		course, err := trainingMgr.GetCourse(record.CourseID)
		courseName := record.CourseID
		if err == nil {
			// UNICODE: Rune-aware truncation preserves multi-byte characters
			courseName = util.TruncateRunes(course.Name, 23)
		}

		completedDate := record.CompletedAt.Format("2006-01-02")
		expiresDate := record.ExpiresAt.Format("2006-01-02")

		statusStr := trainingGreenStyle.Render("PASS")
		if !record.Passed {
			statusStr = trainingRedStyle.Render("FAIL")
		} else if record.IsExpired() {
			statusStr = trainingRedStyle.Render("EXPIRED")
		} else if record.IsExpiringSoon(30 * 24 * time.Hour) {
			statusStr = trainingOrangeStyle.Render("EXPIRING")
		}

		fmt.Printf("  %-25s %-12s %-12s %6.1f%%   %s\n",
			courseName,
			completedDate,
			expiresDate,
			record.Score,
			statusStr)
	}

	fmt.Println()
	fmt.Printf("  Total: %d training record(s)\n", len(history))
	fmt.Println()

	return nil
}

// =============================================================================
// TRAINING EXPIRING
// =============================================================================

// handleTrainingExpiring shows training expiring soon.
func handleTrainingExpiring(args TrainingArgs) error {
	trainingMgr := security.GlobalTrainingManager()
	expiring := trainingMgr.GetExpiringTraining(args.Days)

	if args.JSON {
		data, err := json.MarshalIndent(map[string]interface{}{
			"days":     args.Days,
			"expiring": expiring,
		}, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	fmt.Println()
	fmt.Println(trainingTitleStyle.Render(fmt.Sprintf("Training Expiring Within %d Days", args.Days)))
	fmt.Println(trainingDimStyle.Render(strings.Repeat("=", 70)))
	fmt.Println()

	if len(expiring) == 0 {
		fmt.Println(trainingGreenStyle.Render("  No training expiring soon."))
		fmt.Println()
		return nil
	}

	totalExpiring := 0
	for userID, records := range expiring {
		fmt.Println(trainingSectionStyle.Render(fmt.Sprintf("User: %s", userID)))

		for _, record := range records {
			course, _ := trainingMgr.GetCourse(record.CourseID)
			courseName := record.CourseID
			if course != nil {
				courseName = course.Name
			}

			daysRemaining := int(record.TimeUntilExpiration().Hours() / 24)
			statusColor := trainingYellowStyle
			if daysRemaining <= 7 {
				statusColor = trainingOrangeStyle
			}

			fmt.Printf("  %s%s\n", trainingLabelStyle.Render("Course:"), trainingValueStyle.Render(courseName))
			fmt.Printf("  %s%s\n", trainingLabelStyle.Render("Expires:"), statusColor.Render(record.ExpiresAt.Format("2006-01-02")))
			fmt.Printf("  %s%s\n", trainingLabelStyle.Render("Days Remaining:"), statusColor.Render(fmt.Sprintf("%d days", daysRemaining)))
			fmt.Println()
			totalExpiring++
		}
	}

	fmt.Printf("  Total: %d user(s) with %d expiring course(s)\n", len(expiring), totalExpiring)
	fmt.Println()

	return nil
}

// =============================================================================
// TRAINING REPORT
// =============================================================================

// handleTrainingReport generates a training compliance report.
func handleTrainingReport(args TrainingArgs) error {
	trainingMgr := security.GlobalTrainingManager()
	role := security.UserRole(args.Role)
	report := trainingMgr.GenerateTrainingReport(role)

	if args.JSON {
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	fmt.Println()
	fmt.Println(trainingTitleStyle.Render("AT-2/AT-3 Training Compliance Report"))
	fmt.Println(trainingDimStyle.Render(strings.Repeat("=", 60)))
	fmt.Println()

	// Report metadata
	fmt.Println(trainingSectionStyle.Render("Report Information"))
	fmt.Printf("  %s%s\n", trainingLabelStyle.Render("Generated:"), trainingValueStyle.Render(report.GeneratedAt.Format(time.RFC1123)))
	fmt.Printf("  %s%s\n", trainingLabelStyle.Render("Role:"), trainingValueStyle.Render(string(role)))
	fmt.Println()

	// Summary statistics
	fmt.Println(trainingSectionStyle.Render("Summary Statistics"))
	fmt.Printf("  %s%s\n", trainingLabelStyle.Render("Total Users:"), trainingValueStyle.Render(fmt.Sprintf("%d", report.TotalUsers)))
	fmt.Printf("  %s%s\n", trainingLabelStyle.Render("Compliant:"), trainingGreenStyle.Render(fmt.Sprintf("%d", report.CompliantUsers)))
	fmt.Printf("  %s%s\n", trainingLabelStyle.Render("Non-Compliant:"), trainingRedStyle.Render(fmt.Sprintf("%d", report.NonCompliant)))

	if report.ExpiringTraining > 0 {
		fmt.Printf("  %s%s\n", trainingLabelStyle.Render("Expiring Soon:"), trainingOrangeStyle.Render(fmt.Sprintf("%d courses", report.ExpiringTraining)))
	}

	if report.ExpiredTraining > 0 {
		fmt.Printf("  %s%s\n", trainingLabelStyle.Render("Expired:"), trainingRedStyle.Render(fmt.Sprintf("%d courses", report.ExpiredTraining)))
	}
	fmt.Println()

	// Compliance rate
	complianceRate := 0.0
	if report.TotalUsers > 0 {
		complianceRate = float64(report.CompliantUsers) / float64(report.TotalUsers) * 100
	}

	fmt.Println(trainingSectionStyle.Render("Compliance Rate"))
	rateColor := trainingRedStyle
	if complianceRate >= 90 {
		rateColor = trainingGreenStyle
	} else if complianceRate >= 70 {
		rateColor = trainingYellowStyle
	}
	fmt.Printf("  %s%s\n", trainingLabelStyle.Render("Overall:"), rateColor.Render(fmt.Sprintf("%.1f%%", complianceRate)))
	fmt.Println()

	// User status summary
	if len(report.UserStatuses) > 0 && report.TotalUsers <= 10 {
		fmt.Println(trainingSectionStyle.Render("User Status Details"))
		for userID, status := range report.UserStatuses {
			statusStr := trainingGreenStyle.Render("COMPLIANT")
			if !status.IsCurrent {
				statusStr = trainingRedStyle.Render("NON-COMPLIANT")
			}
			fmt.Printf("  %-20s %s (%d/%d complete)\n",
				userID,
				statusStr,
				status.CompletedCourses,
				status.RequiredCourses)
		}
		fmt.Println()
	}

	// Recommendations
	if report.NonCompliant > 0 || report.ExpiringTraining > 0 {
		fmt.Println(trainingSectionStyle.Render("Recommendations"))
		if report.NonCompliant > 0 {
			fmt.Println(trainingYellowStyle.Render(fmt.Sprintf("  %d user(s) require training completion", report.NonCompliant)))
		}
		if report.ExpiringTraining > 0 {
			fmt.Println(trainingYellowStyle.Render(fmt.Sprintf("  %d course(s) expiring soon - schedule renewals", report.ExpiringTraining)))
		}
		fmt.Println()
	}

	return nil
}

// =============================================================================
// TRAINING COURSES
// =============================================================================

// handleTrainingCourses lists all available training courses.
func handleTrainingCourses(args TrainingArgs) error {
	trainingMgr := security.GlobalTrainingManager()
	courses := trainingMgr.ListCourses()

	if args.JSON {
		data, err := json.MarshalIndent(map[string]interface{}{
			"courses": courses,
			"count":   len(courses),
		}, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	fmt.Println()
	fmt.Println(trainingTitleStyle.Render("Available Training Courses"))
	fmt.Println(trainingDimStyle.Render(strings.Repeat("=", 60)))
	fmt.Println()

	for _, course := range courses {
		typeLabel := string(course.Type)
		if course.Type == security.CourseTypeAwareness {
			typeLabel = "AT-2: Security Awareness"
		} else if course.Type == security.CourseTypeRoleBased {
			typeLabel = "AT-3: Role-Based"
		}

		fmt.Println(trainingSectionStyle.Render(course.Name))
		fmt.Printf("  %s%s\n", trainingLabelStyle.Render("Course ID:"), trainingValueStyle.Render(course.ID))
		fmt.Printf("  %s%s\n", trainingLabelStyle.Render("Type:"), trainingValueStyle.Render(typeLabel))
		fmt.Printf("  %s%s\n", trainingLabelStyle.Render("Duration:"), trainingValueStyle.Render(formatDurationShort(course.Duration)))
		fmt.Printf("  %s%s\n", trainingLabelStyle.Render("Passing Score:"), trainingValueStyle.Render(fmt.Sprintf("%.0f%%", course.PassingScore)))
		fmt.Printf("  %s%s\n", trainingLabelStyle.Render("Valid For:"), trainingValueStyle.Render(formatDurationShort(course.ExpirationPeriod)))

		// Required for roles
		roles := make([]string, len(course.RequiredFor))
		for i, role := range course.RequiredFor {
			roles[i] = string(role)
		}
		fmt.Printf("  %s%s\n", trainingLabelStyle.Render("Required For:"), trainingValueStyle.Render(strings.Join(roles, ", ")))

		fmt.Println()
		fmt.Println(trainingDimStyle.Render("  " + course.Description))
		fmt.Println()
	}

	fmt.Printf("  Total: %d available course(s)\n", len(courses))
	fmt.Println()

	return nil
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

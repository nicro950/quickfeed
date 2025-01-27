package web

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	pb "github.com/autograde/quickfeed/ag"
	"github.com/autograde/quickfeed/assignments"
	"github.com/autograde/quickfeed/scm"
)

var (
	criteriaFile = "criteria.json"
	reviewLayout = "02 Jan 15:04"
)

// getAssignments lists the assignments for the provided course.
func (s *AutograderService) getAssignments(courseID uint64) (*pb.Assignments, error) {
	allAssignments, err := s.db.GetAssignmentsByCourse(courseID, true)
	if err != nil {
		return nil, err
	}
	// Hack to ensure that assignments stored in database with wrong format
	// is displayed correctly in the frontend. This should ideally be removed
	// when the database no longer contains any incorrectly formatted dates.
	for _, assignment := range allAssignments {
		assignment.Deadline = assignments.FixDeadline(assignment.GetDeadline())
	}
	return &pb.Assignments{Assignments: allAssignments}, nil
}

// updateAssignments updates the assignments for the given course.
func (s *AutograderService) updateAssignments(ctx context.Context, sc scm.SCM, courseID uint64) error {
	course, err := s.db.GetCourse(courseID, false)
	if err != nil {
		return err
	}
	assignments, err := assignments.FetchAssignments(ctx, sc, course)
	if err != nil {
		return err
	}
	if err = s.db.UpdateAssignments(assignments); err != nil {
		return err
	}
	return nil
}

func (s *AutograderService) createBenchmark(query *pb.GradingBenchmark) (*pb.GradingBenchmark, error) {
	if _, err := s.db.GetAssignment(&pb.Assignment{
		ID: query.AssignmentID,
	}); err != nil {
		return nil, err
	}
	if err := s.db.CreateBenchmark(query); err != nil {
		return nil, err
	}
	return query, nil
}

func (s *AutograderService) updateBenchmark(query *pb.GradingBenchmark) error {
	return s.db.UpdateBenchmark(query)
}

func (s *AutograderService) deleteBenchmark(query *pb.GradingBenchmark) error {
	return s.db.DeleteBenchmark(query)
}

func (s *AutograderService) createCriterion(query *pb.GradingCriterion) (*pb.GradingCriterion, error) {
	if err := s.db.CreateCriterion(query); err != nil {
		return nil, err
	}
	return query, nil
}

func (s *AutograderService) updateCriterion(query *pb.GradingCriterion) error {
	return s.db.UpdateCriterion(query)
}

func (s *AutograderService) deleteCriterion(query *pb.GradingCriterion) error {
	return s.db.DeleteCriterion(query)
}

// TODO(meling) The criteria.json can be loaded in assignments/FetchAssignments() which already clones the tests repo
func (s *AutograderService) loadCriteria(ctx context.Context, sc scm.SCM, request *pb.AssignmentRequest) ([]*pb.GradingBenchmark, error) {
	query := &pb.Assignment{ID: request.AssignmentID, CourseID: request.CourseID}
	assignment, course, err := s.getAssignmentWithCourse(query, false)
	if err != nil {
		return nil, err
	}

	opts := &scm.FileOptions{
		Path:       filepath.Join(assignment.GetName(), criteriaFile),
		Owner:      course.OrganizationPath,
		Repository: pb.TestsRepo,
	}

	criteriaString, err := sc.GetFileContent(ctx, opts)
	if err != nil {
		return nil, err
	}

	var benchmarks []*pb.GradingBenchmark
	if err := json.Unmarshal([]byte(criteriaString), &benchmarks); err != nil {
		return nil, err
	}

	if len(assignment.GradingBenchmarks) > 0 {
		if err := s.removeOldCriteriaAndReviews(assignment); err != nil {
			return nil, err
		}
	}

	for _, bm := range benchmarks {
		bm.AssignmentID = assignment.ID
		if err := s.db.CreateBenchmark(bm); err != nil {
			return nil, err
		}
		for _, c := range bm.Criteria {
			c.BenchmarkID = bm.ID
			if err := s.db.CreateCriterion(c); err != nil {
				return nil, err
			}
		}
	}

	return benchmarks, nil
}

func (s *AutograderService) createReview(review *pb.Review) (*pb.Review, error) {
	submission, err := s.db.GetSubmission(&pb.Submission{ID: review.SubmissionID})
	if err != nil {
		return nil, err
	}
	assignment, err := s.db.GetAssignment(&pb.Assignment{ID: submission.AssignmentID})
	if err != nil {
		return nil, err
	}
	if len(submission.Reviews) >= int(assignment.Reviewers) {
		return nil, fmt.Errorf("Failed to create a new review for submission %d to assignment %s: all %d reviews already created",
			submission.ID, assignment.Name, assignment.Reviewers)
	}
	review.Edited = time.Now().Format(reviewLayout)
	review.ComputeScore()

	for _, bm := range review.GradingBenchmarks {
		bm.ID = 0
		for _, c := range bm.Criteria {
			c.ID = 0
		}
	}
	if err := s.db.CreateReview(review); err != nil {
		return nil, err
	}

	return review, nil
}

func (s *AutograderService) updateReview(review *pb.Review) (*pb.Review, error) {
	if review.ID == 0 {
		return nil, fmt.Errorf("Cannot update review with empty ID")
	}
	submission, err := s.db.GetSubmission(&pb.Submission{ID: review.SubmissionID})
	if err != nil {
		return nil, err
	}

	review.Edited = time.Now().Format(reviewLayout)
	review.ComputeScore()

	if err := s.db.UpdateReview(review); err != nil {
		return nil, err
	}

	for _, bm := range review.GradingBenchmarks {
		if err := s.db.UpdateBenchmark(bm); err != nil {
			return nil, err
		}
		for _, c := range bm.Criteria {
			if err := s.db.UpdateCriterion(c); err != nil {
				return nil, err
			}
		}
	}
	if submission.Released {
		// Updated review will most probably have a new score. Update the submission score as well
		// for submissions with released review.
		submission.Score = review.Score
		if err := s.db.UpdateSubmission(submission); err != nil {
			return nil, err
		}
	}

	return review, nil
}

func (s *AutograderService) removeOldCriteriaAndReviews(assignment *pb.Assignment) error {
	for _, bm := range assignment.GradingBenchmarks {
		for _, c := range bm.Criteria {
			if err := s.db.DeleteCriterion(c); err != nil {
				fmt.Printf("Failed to delete criteria %v: %s\n", c, err)
			}
		}
		if err := s.db.DeleteBenchmark(bm); err != nil {
			fmt.Printf("Failed to delete benchmark %v: %s\n", bm, err)
		}
	}
	submissions, err := s.db.GetSubmissions(&pb.Submission{AssignmentID: assignment.GetID()})
	if err != nil {
		return err
	}
	for _, submission := range submissions {
		if err := s.db.DeleteReview(&pb.Review{SubmissionID: submission.ID}); err != nil {
			return err
		}
	}
	return nil
}

func (s *AutograderService) getAssignmentWithCourse(query *pb.Assignment, withCourseInfo bool) (*pb.Assignment, *pb.Course, error) {
	assignment, err := s.db.GetAssignment(query)
	if err != nil {
		return nil, nil, err
	}

	course, err := s.db.GetCourse(assignment.CourseID, withCourseInfo)
	if err != nil {
		return nil, nil, err
	}
	return assignment, course, nil
}

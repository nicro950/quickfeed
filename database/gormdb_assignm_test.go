package database_test

import (
	"testing"

	pb "github.com/autograde/aguis/ag"
)

func TestGetNextAssignment(t *testing.T) {
	db, cleanup := setup(t)
	defer cleanup()

	_, err := db.GetNextAssignment(0, 0, 0)
	if err == nil {
		t.Fatal("expected error 'record not found'")
	}

	course := pb.Course{
		Name:        "Distributed Systems",
		Code:        "DAT520",
		Year:        2018,
		Tag:         "Spring",
		Provider:    "fake",
		DirectoryId: 1,
	}

	// create course as teacher
	teacher := createFakeUser(t, db, 10)
	if err := db.CreateCourse(teacher.Id, &course); err != nil {
		t.Fatal(err)
	}

	// create and enroll user as student
	user := createFakeUser(t, db, 11)
	if err := db.CreateEnrollment(&pb.Enrollment{CourseId: course.Id, UserId: user.Id}); err != nil {
		t.Fatal(err)
	}
	if err = db.EnrollStudent(user.Id, course.Id); err != nil {
		t.Fatal(err)
	}

	// create group with single student
	group := pb.Group{
		CourseId: course.Id,
		Users: []*pb.User{
			{Id: user.Id},
		},
	}
	if err := db.CreateGroup(&group); err != nil {
		t.Fatal(err)
	}

	_, err = db.GetNextAssignment(course.Id, user.Id, group.Id)
	if err == nil {
		t.Fatal("expected error 'no assignments found for course 1'")
	}

	// create assignments for course
	assignment1 := pb.Assignment{CourseId: course.Id, Order: 1}
	if err := db.CreateAssignment(&assignment1); err != nil {
		t.Fatal(err)
	}
	assignment2 := pb.Assignment{CourseId: course.Id, Order: 2}
	if err := db.CreateAssignment(&assignment2); err != nil {
		t.Fatal(err)
	}
	assignment3 := pb.Assignment{CourseId: course.Id, Order: 3, IsGrouplab: true}
	if err := db.CreateAssignment(&assignment3); err != nil {
		t.Fatal(err)
	}
	assignment4 := pb.Assignment{CourseId: course.Id, Order: 4}
	if err := db.CreateAssignment(&assignment4); err != nil {
		t.Fatal(err)
	}

	_, err = db.GetNextAssignment(course.Id, 0, 0)
	if err == nil {
		t.Fatal("expected error 'record not found'")
	}

	nxtUnapproved, err := db.GetNextAssignment(course.Id, user.Id, 0)
	if err != nil {
		t.Fatal(err)
	}
	if nxtUnapproved.Id != assignment1.Id {
		t.Errorf("expected unapproved assignment to be %v, got %v", assignment1.Id, nxtUnapproved.Id)
	}

	// send new submission for assignment1
	submission1 := pb.Submission{AssignmentId: assignment1.Id, UserId: user.Id}
	if err := db.CreateSubmission(&submission1); err != nil {
		t.Fatal(err)
	}
	// send another submission for assignment1
	submission2 := pb.Submission{AssignmentId: assignment1.Id, UserId: user.Id}
	if err := db.CreateSubmission(&submission2); err != nil {
		t.Fatal(err)
	}
	// send new submission for assignment2
	submission3 := pb.Submission{AssignmentId: assignment2.Id, UserId: user.Id}
	if err := db.CreateSubmission(&submission3); err != nil {
		t.Fatal(err)
	}
	// send new submission for assignment3
	// submission4 := models.Submission{AssignmentId: assignment3.Id, UserId: user.Id, GroupId: group.Id}
	submission4 := pb.Submission{AssignmentId: assignment3.Id, GroupId: group.Id}
	if err := db.CreateSubmission(&submission4); err != nil {
		t.Fatal(err)
	}

	// we haven't approved any of the submissions yet; expect same result as above

	nxtUnapproved, err = db.GetNextAssignment(course.Id, user.Id, 0)
	if err != nil {
		t.Fatal(err)
	}
	if nxtUnapproved.Id != assignment1.Id {
		t.Errorf("expected unapproved assignment to be %v, got %v", assignment1.Id, nxtUnapproved.Id)
	}

	// approve submission1
	if err := db.UpdateSubmissionByID(submission1.Id, true); err != nil {
		t.Fatal(err)
	}

	// we have approved the first submission of the first assignment, but since
	// we two submissions for assignment1, this won't change the next to approve.
	// TODO(meling) Is this the desired semantics for this??
	// That is, it seems more reasonable to have a function ApproveAssignment(assignment, user)
	// that finds the latest submission for the user and marks it approved.
	// That is, maybe the UpdateSubmissionById shouldn't be exported.
	nxtUnapproved, err = db.GetNextAssignment(course.Id, user.Id, 0)
	if err != nil {
		t.Fatal(err)
	}
	if nxtUnapproved.Id != assignment1.Id {
		t.Errorf("expected unapproved assignment to be %v, got %v", assignment1.Id, nxtUnapproved.Id)
	}

	// approve submission2
	if err := db.UpdateSubmissionByID(submission2.Id, true); err != nil {
		t.Fatal(err)
	}

	// now the first assignment is approved, moving on to the second
	nxtUnapproved, err = db.GetNextAssignment(course.Id, user.Id, 0)
	if err != nil {
		t.Fatal(err)
	}
	if nxtUnapproved.Id != assignment2.Id {
		t.Errorf("expected unapproved assignment to be %v, got %v", assignment2.Id, nxtUnapproved.Id)
	}

	// approve submission3
	if err := db.UpdateSubmissionByID(submission3.Id, true); err != nil {
		t.Fatal(err)
	}

	// now the second assignment is approved, moving on to the third
	// this fails because the next assignment to approve is a group lab,
	// and we don't provide a group id.
	_, err = db.GetNextAssignment(course.Id, user.Id, 0)
	//TODO(meling) GetNextAssignment semantics has changed; needs to be updated when we understand better what is needed
	// if err == nil {
	// 	t.Fatal("expected error 'record not found'")
	// }

	// moving on to the third assignment, using the group Id this time.
	// fails because user id must be provided.
	_, err = db.GetNextAssignment(course.Id, 0, group.Id)
	//TODO(meling) GetNextAssignment semantics has changed; needs to be updated when we understand better what is needed
	// if err == nil {
	// 	t.Fatal("expected error 'user id must be provided'")
	// }

	// moving on to the third assignment, using both user id and group id this time.
	nxtUnapproved, err = db.GetNextAssignment(course.Id, user.Id, group.Id)
	if err != nil {
		t.Fatal(err)
	}
	if nxtUnapproved.Id != assignment3.Id {
		t.Errorf("expected unapproved assignment to be %v, got %v", assignment3.Id, nxtUnapproved.Id)
	}

	// approve submission4 for assignment3 (the group lab)
	if err := db.UpdateSubmissionByID(submission4.Id, true); err != nil {
		t.Fatal(err)
	}

	// approving the 4th submission (for assignment3, which is a group lab),
	// should fail because we only provIde user Id, and no group.Id.
	_, err = db.GetNextAssignment(course.Id, user.Id, 0)
	//TODO(meling) GetNextAssignment semantics has changed; needs to be updated when we understand better what is needed
	// if err == nil {
	// 	t.Fatal("expected error 'user id must be provided'")
	// }

	// here it should pass since we also provide the group id.
	nxtUnapproved, err = db.GetNextAssignment(course.Id, user.Id, group.Id)
	if err != nil {
		t.Fatal(err)
	}
	if nxtUnapproved.Id != assignment4.Id {
		t.Errorf("expected unapproved assignment to be %v, got %v", assignment4.Id, nxtUnapproved.Id)
	}

	// send new submission for assignment4
	submission5 := pb.Submission{AssignmentId: assignment4.Id, UserId: user.Id}
	if err := db.CreateSubmission(&submission5); err != nil {
		t.Fatal(err)
	}
	// approve submission5
	if err := db.UpdateSubmissionByID(submission5.Id, true); err != nil {
		t.Fatal(err)
	}
	// all assignments have been approved
	nxtUnapproved, err = db.GetNextAssignment(course.Id, user.Id, group.Id)
	if nxtUnapproved != nil || err == nil {
		t.Fatal("expected error 'all assignments approved'")
	}
}

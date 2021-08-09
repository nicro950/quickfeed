import * as React from "react";
import { LabResult, LastBuild, LastBuildInfo, Row } from "../../components";
import { ISubmissionLink, ISubmission } from "../../models";
import { User, Submission } from "../../../proto/ag/ag_pb";
import { Release } from "../../components/manual-grading/Release";
interface ILabInfoProps {
    submissionLink: ISubmissionLink;
    student: User;
    courseURL: string;
    slipdays: number;
    teacherPageView: boolean;
    onSubmissionStatusUpdate: (status: Submission.Status) => void;
    onSubmissionRebuild: (assignmentID: number, submissionID: number) => Promise<boolean>;
    onSubmissionBuild: (assignmentID: number) => Promise<boolean>;
}

export class LabResultView extends React.Component<ILabInfoProps> {

    public render() {
        console.log(this.props);
        if (this.props.submissionLink.submission) {
            const currentSubmission = this.props.submissionLink.submission;
            const buildLog = currentSubmission.buildInfo.getBuildlog().split("\n").map((x, i) => <span key={i} >{x}<br /></span>);
            const lastBuildTable = (<LastBuild
            test_cases={currentSubmission.testCases}
            score={currentSubmission.score}
            scoreLimit={this.props.submissionLink.assignment.getScorelimit()}
            weight={100}
        />)
            return (
                <div key="labhead" className="col-md-9 col-sm-9 col-xs-12">
                    <div key="labview" className="result-content" id="resultview">
                        <section id="result">
                            <LabResult
                                assignmentID={this.props.submissionLink.assignment.getId()}
                                submissionID={currentSubmission.id}
                                scoreLimit={this.props.submissionLink.assignment.getScorelimit()}
                                teacherView={this.props.teacherPageView}
                                lab={this.props.submissionLink.assignment.getName()}
                                progress={currentSubmission.score}
                                status={currentSubmission.status}
                                authorName={this.props.submissionLink.authorName}
                                onSubmissionStatusUpdate={this.props.onSubmissionStatusUpdate}
                                onSubmissionRebuild={this.props.onSubmissionRebuild}
                            />
                            <LastBuildInfo
                                submission={currentSubmission}
                                slipdays={this.props.slipdays}
                                assignment={this.props.submissionLink.assignment}
                                teacherView={this.props.teacherPageView}
                            />
                            {this.props.submissionLink.assignment.getReviewers() > 0 ? null : lastBuildTable}
                            {this.props.submissionLink.assignment.getReviewers() > 0 && currentSubmission.released ? this.renderReviewInfo(currentSubmission) : null}
                            <Row><div key="loghead" className="col-lg-12"><div key="logview" className="well"><code id="logs">{buildLog}</code></div></div></Row>
                        </section>
                    </div>
                </div>
            );
        }
        return (
            <div>
                <h1>No submissions yet</h1>
                <button type="button" className="btn btn-primary" onClick={() => {this.buildSubmission()}}>First build</button>
            </div>
        );
    }

    private async buildSubmission() {
        await this.props.onSubmissionBuild(this.props.submissionLink.assignment.getId())
    }

    private reviewersForStudentPage(submission: ISubmission): User[] {
        const reviewers: User[] = [];
        submission.reviews.forEach(r => {
            if (r.getReady()) {
                const reviewer = new User();
                reviewer.setId(r.getReviewerid());
                reviewers.push(reviewer);
            }
        });
        return reviewers;
    }

    private renderReviewInfo(submission: ISubmission): JSX.Element {
        if (this.props.teacherPageView) {
            return <div className="row">
            </div>
        }
        return <Release
            submission={submission}
            assignment={this.props.submissionLink.assignment}
            userIsCourseCreator={false}
            authorName={this.props.student.getName()}
            authorLogin={this.props.student.getLogin()}
            studentNumber={0}
            courseURL={this.props.courseURL}
            teacherView={false}
            isSelected={true}
            setGrade={async () => { return false }}
            release={() => { return }}
            getReviewers={async () => {return this.reviewersForStudentPage(submission)}}
        />
    }
}

package github

// GraphQL schema fragments for GitHub Projects API v2
// These fragments are used across multiple queries to ensure consistency

// ProjectV2ItemFieldValueFragment represents common field value structures
const ProjectV2ItemFieldValueFragment = `
fragment ProjectV2ItemFieldValueFragment on ProjectV2ItemFieldValue {
	... on ProjectV2ItemFieldTextValue {
		text
		field {
			... on ProjectV2FieldCommon {
				name
			}
		}
	}
	... on ProjectV2ItemFieldNumberValue {
		number
		field {
			... on ProjectV2FieldCommon {
				name
			}
		}
	}
	... on ProjectV2ItemFieldDateValue {
		date
		field {
			... on ProjectV2FieldCommon {
				name
			}
		}
	}
	... on ProjectV2ItemFieldSingleSelectValue {
		name
		field {
			... on ProjectV2FieldCommon {
				name
			}
		}
	}
	... on ProjectV2ItemFieldIterationValue {
		title
		startDate
		duration
		field {
			... on ProjectV2FieldCommon {
				name
			}
		}
	}
	... on ProjectV2ItemFieldRepositoryValue {
		repository {
			name
			owner {
				login
			}
		}
		field {
			... on ProjectV2FieldCommon {
				name
			}
		}
	}
	... on ProjectV2ItemFieldUserValue {
		users(first: 10) {
			nodes {
				login
			}
		}
		field {
			... on ProjectV2FieldCommon {
				name
			}
		}
	}
	... on ProjectV2ItemFieldLabelValue {
		labels(first: 10) {
			nodes {
				name
			}
		}
		field {
			... on ProjectV2FieldCommon {
				name
			}
		}
	}
	... on ProjectV2ItemFieldMilestoneValue {
		milestone {
			title
		}
		field {
			... on ProjectV2FieldCommon {
				name
			}
		}
	}
	... on ProjectV2ItemFieldPullRequestValue {
		pullRequests(first: 10) {
			nodes {
				title
				number
			}
		}
		field {
			... on ProjectV2FieldCommon {
				name
			}
		}
	}
}
`

// ProjectV2FieldFragment represents field definitions
const ProjectV2FieldFragment = `
fragment ProjectV2FieldFragment on ProjectV2FieldConfiguration {
	... on ProjectV2Field {
		id
		name
		dataType
	}
	... on ProjectV2SingleSelectField {
		id
		name
		dataType
		options {
			id
			name
			color
			description
		}
	}
	... on ProjectV2IterationField {
		id
		name
		dataType
		configuration {
			duration
			startDay
			iterations {
				id
				title
				startDate
				duration
			}
		}
	}
}
`

// IssueFragment represents issue content fields
const IssueFragment = `
fragment IssueFragment on Issue {
	id
	number
	title
	body
	state
	url
	createdAt
	updatedAt
	closedAt
	author {
		login
	}
	assignees(first: 10) {
		nodes {
			login
		}
	}
	labels(first: 10) {
		nodes {
			name
			color
		}
	}
	milestone {
		title
		dueOn
		state
	}
	repository {
		name
		owner {
			login
		}
	}
}
`

// PullRequestFragment represents pull request content fields
const PullRequestFragment = `
fragment PullRequestFragment on PullRequest {
	id
	number
	title
	body
	state
	url
	createdAt
	updatedAt
	closedAt
	mergedAt
	author {
		login
	}
	assignees(first: 10) {
		nodes {
			login
		}
	}
	labels(first: 10) {
		nodes {
			name
			color
		}
	}
	milestone {
		title
		dueOn
		state
	}
	repository {
		name
		owner {
			login
		}
	}
	headRefName
	baseRefName
	isDraft
	mergeable
	additions
	deletions
	changedFiles
}
`

// ProjectV2ItemFragment represents a project card/item
const ProjectV2ItemFragment = `
fragment ProjectV2ItemFragment on ProjectV2Item {
	id
	type
	createdAt
	updatedAt
	isArchived
	creator {
		login
	}
	content {
		... on Issue {
			...IssueFragment
		}
		... on PullRequest {
			...PullRequestFragment
		}
		... on DraftIssue {
			title
			body
			createdAt
			updatedAt
			creator {
				login
			}
		}
	}
	fieldValues(first: 20) {
		nodes {
			...ProjectV2ItemFieldValueFragment
		}
	}
}
`

// Helper functions to build complete queries with fragments

// BuildProjectCardsQuery builds a complete query for listing project cards
func BuildProjectCardsQuery(includeContent bool) string {
	if includeContent {
		return IssueFragment + "\n" + PullRequestFragment + "\n" + ProjectV2ItemFieldValueFragment + "\n" + ProjectV2ItemFragment
	}
	return ProjectV2ItemFieldValueFragment + "\n" + `
fragment ProjectV2ItemBasicFragment on ProjectV2Item {
	id
	type
	createdAt
	updatedAt
	isArchived
	fieldValues(first: 20) {
		nodes {
			...ProjectV2ItemFieldValueFragment
		}
	}
}`
}

// BuildProjectFieldsQuery builds a complete query for project fields
func BuildProjectFieldsQuery() string {
	return ProjectV2FieldFragment
}
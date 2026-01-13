package swarm

// Role represents an agent's role in the swarm.
type Role string

// Predefined roles for swarm agents.
const (
	RoleOrchestrator Role = "ORCH"
	RoleSA           Role = "SA"
	RoleBEDev        Role = "BE_DEV"
	RoleFEDev        Role = "FE_DEV"
	RoleQA           Role = "QA"
	RoleDevOps       Role = "DEVOPS"
	RoleDBA          Role = "DBA"
	RoleSecurity     Role = "SEC"
	RoleHuman        Role = "HUMAN"
	RoleAll          Role = "ALL" // Special role for broadcasts
)

// RoleDefinition contains the full definition of a role including its behavior.
type RoleDefinition struct {
	Role         Role     `json:"role" yaml:"role"`
	Name         string   `json:"name" yaml:"name"`
	Description  string   `json:"description" yaml:"description"`
	SystemPrompt string   `json:"system_prompt" yaml:"system_prompt"`
	Capabilities []string `json:"capabilities" yaml:"capabilities"`   // What the role can do
	Permissions  []string `json:"permissions" yaml:"permissions"`     // Who the role can message
	CanInitiate  bool     `json:"can_initiate" yaml:"can_initiate"`   // Can start conversations
	CanApprove   bool     `json:"can_approve" yaml:"can_approve"`     // Can approve PRs, designs
	CanExecute   bool     `json:"can_execute" yaml:"can_execute"`     // Can run code, make changes
	AskBefore    []Role   `json:"ask_before" yaml:"ask_before"`       // Roles to consult before acting
	Tools        []string `json:"tools,omitempty" yaml:"tools"`       // Allowed tools (empty = all)
}

// String returns the string representation of the role.
func (r Role) String() string {
	return string(r)
}

// IsValid checks if the role is a known role.
func (r Role) IsValid() bool {
	switch r {
	case RoleOrchestrator, RoleSA, RoleBEDev, RoleFEDev, RoleQA,
		RoleDevOps, RoleDBA, RoleSecurity, RoleHuman, RoleAll:
		return true
	default:
		return false
	}
}

// IsBroadcast checks if this role represents a broadcast target.
func (r Role) IsBroadcast() bool {
	return r == RoleAll
}

// DefaultRoles returns the built-in role definitions.
// It first tries to load from embedded markdown files, then falls back to hardcoded defaults.
func DefaultRoles() map[Role]*RoleDefinition {
	// Try loading from embedded files
	roles, err := LoadRoles("")
	if err == nil && len(roles) > 0 {
		// Set permissions based on role
		for role, def := range roles {
			setDefaultPermissions(role, def)
		}
		return roles
	}

	// Fallback to hardcoded defaults
	return hardcodedDefaults()
}

// setDefaultPermissions sets the can* flags based on role
func setDefaultPermissions(role Role, def *RoleDefinition) {
	switch role {
	case RoleOrchestrator:
		def.CanInitiate = true
		def.CanApprove = true
		def.CanExecute = false
	case RoleSA:
		def.CanInitiate = false
		def.CanApprove = true
		def.CanExecute = false
		def.AskBefore = []Role{RoleOrchestrator}
	case RoleBEDev:
		def.CanInitiate = false
		def.CanApprove = false
		def.CanExecute = true
		def.AskBefore = []Role{RoleSA, RoleOrchestrator}
	case RoleFEDev:
		def.CanInitiate = false
		def.CanApprove = false
		def.CanExecute = true
		def.AskBefore = []Role{RoleSA, RoleOrchestrator}
	case RoleQA:
		def.CanInitiate = false
		def.CanApprove = true
		def.CanExecute = true
		def.AskBefore = []Role{RoleOrchestrator}
	case RoleDevOps:
		def.CanInitiate = false
		def.CanApprove = false
		def.CanExecute = true
		def.AskBefore = []Role{RoleOrchestrator}
	case RoleDBA:
		def.CanInitiate = false
		def.CanApprove = true
		def.CanExecute = true
		def.AskBefore = []Role{RoleSA, RoleOrchestrator}
	case RoleSecurity:
		def.CanInitiate = false
		def.CanApprove = true
		def.CanExecute = false
		def.AskBefore = []Role{RoleOrchestrator}
	case RoleHuman:
		def.CanInitiate = true
		def.CanApprove = true
		def.CanExecute = true
	}
}

// hardcodedDefaults returns fallback role definitions
func hardcodedDefaults() map[Role]*RoleDefinition {
	return map[Role]*RoleDefinition{
		RoleOrchestrator: {
			Role:        RoleOrchestrator,
			Name:        "Orchestrator",
			Description: "Breaks down tasks, coordinates agents, makes decisions",
			CanInitiate: true,
			CanApprove:  true,
			CanExecute:  false,
			SystemPrompt: `You are the Orchestrator of a development swarm. Your responsibilities:

1. **Task Decomposition**: Break user requests into specific tasks for each role
2. **Coordination**: Assign work, track progress, resolve blockers
3. **Decision Making**: Approve designs, resolve conflicts between agents
4. **Quality Gate**: Review completed work before marking tasks done

When you receive a user request:
1. Analyze what roles are needed (SA for design, BE_DEV for backend, etc.)
2. Create a clear task breakdown and announce to @ALL
3. Assign specific tasks to specific roles
4. Monitor progress and help agents who are stuck

You can use these commands:
- @SA, @BE_DEV, @FE_DEV, @QA, etc. - Direct message to a role
- @ALL - Broadcast to all agents
- APPROVE: <message> - Approve a proposal or PR
- REJECT: <message> - Reject with feedback`,
		},
		RoleSA: {
			Role:        RoleSA,
			Name:        "Solution Architect",
			Description: "Designs systems, defines schemas, answers 'how should we build this?'",
			CanInitiate: false,
			CanApprove:  true,
			CanExecute:  false,
			AskBefore:   []Role{RoleOrchestrator},
			SystemPrompt: `You are the Solution Architect. Your responsibilities:

1. **System Design**: Design APIs, database schemas, architecture
2. **Technical Decisions**: Choose libraries, patterns, approaches
3. **Documentation**: Create design docs, diagrams, ADRs
4. **Guidance**: Answer technical questions from developers

When asked to design something:
1. Propose a design to @ORCH for approval
2. Once approved, share detailed specs with implementing roles
3. Answer questions from BE_DEV, FE_DEV, etc.
4. Review implementations against the design

You should NOT write implementation code. Delegate to BE_DEV/FE_DEV.`,
		},
		RoleBEDev: {
			Role:        RoleBEDev,
			Name:        "Backend Developer",
			Description: "Implements APIs, services, database logic",
			CanInitiate: false,
			CanApprove:  false,
			CanExecute:  true,
			AskBefore:   []Role{RoleSA, RoleOrchestrator},
			SystemPrompt: `You are a Backend Developer. Your responsibilities:

1. **Implementation**: Write backend code (APIs, services, database)
2. **Testing**: Write unit tests, integration tests
3. **Code Quality**: Follow project patterns, lint, format

When you receive a task:
1. If the design is unclear, @SA for clarification
2. Implement the feature using available tools (read_file, write_file, bash)
3. Run tests to verify your implementation
4. @QA for code review when ready
5. Report completion to @ORCH

If you encounter issues:
- Technical questions → @SA
- Blockers or scope changes → @ORCH
- Need frontend coordination → @FE_DEV`,
		},
		RoleFEDev: {
			Role:        RoleFEDev,
			Name:        "Frontend Developer",
			Description: "Implements UI components, client-side logic",
			CanInitiate: false,
			CanApprove:  false,
			CanExecute:  true,
			AskBefore:   []Role{RoleSA, RoleOrchestrator},
			SystemPrompt: `You are a Frontend Developer. Your responsibilities:

1. **Implementation**: Write frontend code (components, pages, styling)
2. **Testing**: Write component tests, e2e tests
3. **UX**: Ensure good user experience and accessibility

When you receive a task:
1. If the design is unclear, @SA for clarification
2. Implement the feature using available tools
3. Test in browser/preview
4. @QA for review when ready
5. Report completion to @ORCH

If you encounter issues:
- Technical questions → @SA
- API questions → @BE_DEV
- Blockers or scope changes → @ORCH`,
		},
		RoleQA: {
			Role:        RoleQA,
			Name:        "Quality Assurance",
			Description: "Writes tests, reviews code, finds bugs",
			CanInitiate: false,
			CanApprove:  true,
			CanExecute:  true,
			AskBefore:   []Role{RoleOrchestrator},
			SystemPrompt: `You are a Quality Assurance engineer. Your responsibilities:

1. **Code Review**: Review code for bugs, security issues, best practices
2. **Testing**: Write and run tests
3. **Bug Reporting**: Document issues clearly
4. **Verification**: Verify fixes and implementations

When you receive a review request:
1. Read the code changes carefully
2. Check for bugs, security issues, edge cases
3. Verify tests exist and pass
4. Provide actionable feedback
5. APPROVE or REQUEST_CHANGES with specific items`,
		},
		RoleDevOps: {
			Role:        RoleDevOps,
			Name:        "DevOps Engineer",
			Description: "CI/CD, infrastructure, deployment",
			CanInitiate: false,
			CanApprove:  false,
			CanExecute:  true,
			AskBefore:   []Role{RoleOrchestrator},
			SystemPrompt: `You are a DevOps Engineer. Your responsibilities:

1. **CI/CD**: Set up and maintain pipelines
2. **Infrastructure**: Configure servers, containers, cloud resources
3. **Deployment**: Deploy applications safely
4. **Monitoring**: Set up logging and alerting

When you receive a task:
1. Understand the requirements
2. Implement infrastructure changes
3. Test deployment pipelines
4. Document any manual steps needed`,
		},
		RoleDBA: {
			Role:        RoleDBA,
			Name:        "Database Administrator",
			Description: "Schema design, queries, optimization",
			CanInitiate: false,
			CanApprove:  true,
			CanExecute:  true,
			AskBefore:   []Role{RoleSA, RoleOrchestrator},
			SystemPrompt: `You are a Database Administrator. Your responsibilities:

1. **Schema Design**: Design efficient database schemas
2. **Query Optimization**: Optimize slow queries
3. **Migrations**: Write safe database migrations
4. **Data Integrity**: Ensure data consistency

When you receive a task:
1. Review the data requirements
2. Design or optimize the schema
3. Write migrations with rollback plans
4. Coordinate with @BE_DEV for application changes`,
		},
		RoleSecurity: {
			Role:        RoleSecurity,
			Name:        "Security Engineer",
			Description: "Security review, auth design, vulnerability analysis",
			CanInitiate: false,
			CanApprove:  true,
			CanExecute:  false,
			AskBefore:   []Role{RoleOrchestrator},
			SystemPrompt: `You are a Security Engineer. Your responsibilities:

1. **Security Review**: Review code and architecture for vulnerabilities
2. **Auth Design**: Design authentication and authorization systems
3. **Threat Modeling**: Identify potential attack vectors
4. **Compliance**: Ensure security best practices

When you receive a review request:
1. Check for OWASP Top 10 vulnerabilities
2. Review authentication and authorization
3. Check for data exposure risks
4. Provide specific remediation steps`,
		},
		RoleHuman: {
			Role:        RoleHuman,
			Name:        "Human Observer",
			Description: "Can send messages to any agent, override decisions",
			CanInitiate: true,
			CanApprove:  true,
			CanExecute:  true,
			SystemPrompt: `You are observing the swarm as a human participant. You can:

1. Send messages to any agent using @ROLE
2. Override agent decisions
3. Provide guidance and corrections
4. Take over any role temporarily`,
		},
	}
}

// ParseRole converts a string to a Role, returning an error if invalid.
func ParseRole(s string) (Role, error) {
	r := Role(s)
	if !r.IsValid() {
		return "", ErrInvalidRole
	}
	return r, nil
}

// AllRoles returns a slice of all valid roles (excluding ALL).
func AllRoles() []Role {
	return []Role{
		RoleOrchestrator,
		RoleSA,
		RoleBEDev,
		RoleFEDev,
		RoleQA,
		RoleDevOps,
		RoleDBA,
		RoleSecurity,
		RoleHuman,
	}
}

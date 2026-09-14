# ADR-0007: The local git clone is the only data source

**Status:** Accepted

## Context
Hosting provider APIs would add pull request lifecycle, review timing and issue
linkage. They would also require per-host adapters, credential storage, and
rate-limit management, and would reintroduce the credential custody problem
avoided in ADR-0005. Reading only the clone keeps the tool identical across
GitHub, GitLab, Bitbucket, Gitea, Azure DevOps and self-hosted servers, and
requires no authorisation from the user.

## Decision
1. All metrics MUST be computed from a local git clone and its working tree.
2. The product MUST NOT call a hosting provider's API for analysis data.
3. The product MUST NOT contain host-specific analysis code. Host-specific code
   is permitted only for cloning a remote URL, which is generic git behaviour.
4. The collection stage MUST be implemented behind an interface that admits
   additional collectors, so that a provider-API collector can be added later
   without restructuring the pipeline.
5. Any future provider-API collector MUST be optional. The product MUST remain
   fully functional with the git collector alone.

## Consequences
- Pull request cycle time, review latency, and issue linkage are not available
  and MUST NOT appear in the metric catalogue.
- The product works identically on any git host, including hosts with no API.
- No user is ever asked to authorise access to a hosting account.

## Out of scope
- Pull request, review, issue tracker, CI/CD, incident and calendar data
  sources are not implemented.

## Assumption
The metrics that carry the product's value are derivable from git history.

## Acceptance criteria
- No HTTP client in the analysis path targets a hosting provider API.
- The collection stage is reachable through an interface with at least one
  implementation, and the rest of the pipeline depends on the interface rather
  than the concrete git implementation.

## Dependencies
ADR-0020 must be implemented before this one.

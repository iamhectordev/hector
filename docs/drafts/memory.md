# Organizational Memory

## 1. What This Is

Organizational memory is the system by which what an org has learned — decisions made, failures experienced, practices established, structure evolved — is available to agents and humans without requiring anyone to explicitly write it down or remember to share it.

The unit of truth is the organization, not the user. A decision made in a Slack thread three years ago by a team that no longer exists is org knowledge. A practice that emerged from two consecutive incidents is org knowledge. Who owns what service and why is org knowledge. None of this is personal preference or session history.

**Why existing systems fail.** Wikis, Confluence, Notion, and knowledge management systems all depend on humans deliberately externalizing knowledge — writing things down, keeping them current, tagging them usefully. This is high-friction, rarely done, and produces content that decays within months. The result: orgs pay for knowledge bases that are simultaneously too large (full of stale content) and too empty (missing the tacit knowledge that actually matters).

RAG over a document store improves search but inherits the decay problem. If the document is wrong, the retrieval is confidently wrong.

**The approach here.** Extract knowledge passively from ambient signals (Slack, GitHub, Jira, deploy configs, meeting transcripts). Process it with provenance. Track supersession so stale facts don't surface as current. Build the knowledge base from what the org actually does, not what it intends to document.

**Prior art.** See `~/research/agentmem/` for detailed analysis. Key references: Honcho (user memory architecture, consolidation pipeline), Zep (bitemporal knowledge graph), Atlassian Teamwork Graph (most serious enterprise attempt), Portent (entity taxonomy).

---

## 2. Capabilities

These are the specific things the system must do. Retrieval is not a capability — it is an implementation concern. The capabilities are what retrieval serves.

**A. Answer "why" questions about org artifacts.**
"Why does the auth service use JWT instead of sessions?" requires decision records with rationale, not just the fact that JWT is used. The system must store the decision, who made it, what alternatives were considered, and what the reasoning was.

**B. Surface precedents and what was learned.**
"Have we tried this approach before?" searches event history by domain and topic, returns what happened, and — critically — what was learned. A precedent without a lesson is just a fact.

**C. Current state of practices.**
"What's our current oncall policy?" must respect supersession. An archived runbook from 2023 is not the current policy even if it ranks highest by embedding similarity. The system must know what is current and what has been replaced.

**D. Find relevant people and expertise.**
"Who has worked on the payments system?" and "Who would know about Kafka?" require a person-entity graph where past involvement in projects and topics is traversable.

**E. Pattern surfacing.**
"What are our recurring problems in the data pipeline?" requires consolidation output — inductive observations synthesized across many events — not a search over raw events. This is a higher-order capability that depends on the consolidation process running well.

**F. Detect documented absence.**
"Do we have a policy on gRPC?" when nothing exists in org memory must return "no documented position found," not general knowledge about gRPC. This requires a coverage model: the system must know what domains have been ingested and distinguish "nothing found" from "this hasn't been captured."

**G. Ambient context injection — the primary use case.**
When Hector is invoked in any context, it resolves the entities in that context and loads the relevant memory slice before acting. The human does not need to provide context; Hector already has it.

- **Slack channel mention**: resolve channel → project mapping, load project context (active events, decisions, known issues, ownership).
- **GitHub issue**: extract entity references from title and body (e.g., "auth component"), traverse relations, load what the system knows about those entities.
- **Investigate a service**: know where the logs are, what trace store is in use, which environments are active (and which are decommissioned), who owns it, what's been recently changed or flagged.

This is what separates a useful agent from a generic one. The operational knowledge (log sinks, trace stores, environment inventory) is not a skill — it's memory. Skills are mechanisms ("how to fetch logs from Datadog"). Memory is org-specific state ("service X logs are in Datadog under index `payments-prod`; the dev environment was decommissioned in March"). Encoding org-specific state into skills produces skills that rot silently.

---

## 3. Entity Model

The entity model is based on [Portent](https://github.com/refactoringhq/portent), an open specification for knowledge bases. Portent defines eight types in two groups.

**PORT types** — actionable:
- **Project** — a bounded effort with an output and success criteria. Has a beginning and an end.
- **Operation** — recurring work that can usually be completed in one sitting. Oncall rotations, deploy procedures, review cycles.
- **Responsibility** — a long-running area of accountability. Measured by indicators, not completion. Teams own Responsibilities.
- **Task** — one-off work. May live in an external task tracker; referenced from memory, not duplicated.

**ENTP types** — non-actionable records:
- **Event** — anything that happened and should be retained. Decisions, incidents, launches, conversations, reorgs, achievements. An Event is the general form; the type of event is expressed through attributes, not a separate type.
- **Note** — a durable knowledge artifact. Runbooks, ADRs, reference docs, research summaries, checklists.
- **Topic** — a conceptual lens. Authentication, observability, latency. Collects related Notes and Events without implying ownership or completion.
- **Person** — a real-world person or an agent treated as an actor. Contacts, collaborators, decision-makers.

**Asset** — a Portent extension for things that exist, operate, and can be in good or bad states: software services, data pipelines, infrastructure components, external systems. Assets have owners (related to a Responsibility), dependencies (related to other Assets), and an operational lifecycle (active, degraded, decommissioned).

**Type mapping for common org entities.**

| Entity | Type | Rationale |
|--------|------|-----------|
| Running service (`auth-service`) | Asset | Has logs, envs, owners, operational lifecycle |
| Code repository | Asset | Exists, has owners, history, is deployed |
| External library (`react`, `grpc`) | Topic | Not owned or operated; collect knowledge *about* it |
| External service used operationally (`datadog`) | Topic → Asset | Start as Topic; promote to Asset if org-specific operational facts accumulate |
| Team or squad | Responsibility | The team is their accountability area; members are Persons related to it |
| Recurring process (`incident response`, `oncall`) | Operation | Repeatable work with defined shape |
| Business or workflow process | Operation | Same |

**Topic as the safe catch-all.** When the extractor cannot confidently classify an entity — "Kafka" might be an Asset (we run it) or a Topic (we use a managed version) — it defaults to Topic. Topic makes no ownership or operational claims. It can be promoted to a more specific type later as context accumulates. This is the `captured` lifecycle state applied to entities: the reference is recorded, the characterization is deferred.

**Entity classification heuristics.** The extractor uses the type taxonomy and the existing entity graph as context. Heuristics narrow the options before the LLM decides:
- Ends in `-service`, `-api`, `-worker`, `-db`, `-pipeline` → likely Asset
- "team", "squad", "guild", "group" → likely Responsibility
- Recognizable person name → Person
- External product or library name without deployment context → Topic
- "process", "procedure", "rotation", "workflow" → Operation

**The alias problem.** "auth-service", "auth service", "the auth microservice", "auth", "authentication service" may all refer to the same Asset. The extractor must resolve new mentions against the existing entity graph before creating a new object — fuzzy name matching plus LLM disambiguation. Creating duplicate entities for the same real-world thing produces a fragmented graph where knowledge about one alias is invisible when querying another. This is the most common failure mode in knowledge graph construction and must be addressed at ingestion time, not cleaned up later.

**Relationships.** Two defaults from Portent, plus two for provenance:
- `belongs_to` — primary context or ownership. A Note belongs to the Project it primarily supports. An Event belongs to the Responsibility it occurred under.
- `related_to` — secondary association. A meeting Event is related to its attendees (Persons) and topics (Topics).
- `supersedes` — explicit replacement. When a practice changes, the new Note supersedes the old one. The old Note is archived, not deleted, and the link is traversable.
- `derived_from` — provenance for synthesized observations. Every deductive or inductive observation links to the source observations it was derived from.

**Lifecycle.** Three states from Portent:
- **Captured** — recorded but not yet processed or connected.
- **Organized** — has a type, relationships, and is retrievable.
- **Archived** — no longer active but retained for historical reference. Superseded facts are archived, not deleted.

---

## 4. Provenance Model

Every observation carries how it was obtained. This determines what the agent does with it.

**Evidence types:**
- **Artifact** — extracted from code, config, schema, or a committed document. High reliability. The deploy config says the log sink is Datadog: this is an artifact-level fact.
- **Measured** — extracted from logs, metrics, or traces. High reliability, time-bounded. The error rate for service X was elevated from 14:00–14:47 on 2026-05-20.
- **Reported** — someone stated it in Slack, a meeting, an issue comment. A fact about the utterance, not about the world. "Jack said the admin API is broken" is a reported observation. It does not become "the admin API is broken."
- **Inferred** — derived from other observations through reasoning. Only as reliable as its inputs and the reasoning step.
- **Synthesized** — induced from patterns across many observations. Probabilistic. Should be treated as a hypothesis until validated.

**Storing the utterance, not the claim.** A reported observation is stored as an Event: "Jack said in #engineering that the admin API is broken." A separate Note saying "the admin API is broken" is only created when corroborated by artifact or measured evidence, and it cites that evidence.

This distinction drives agent behavior. When a query returns only reported evidence, the agent qualifies its response: "Jack reported this on 2026-05-12. It has not been verified against metrics or code. Want me to check?" When the evidence is an artifact, the agent reports it as fact and cites the source.

**Supersession without bitemporal timestamps.** Tracking `valid_from` and `valid_to` requires knowing things that are rarely knowable at ingestion time. Instead: new observations that contradict existing ones create a `supersedes` relationship. The superseded observation is archived. The system always traverses to the current (non-superseded) state. The fact that something used to be true is preserved in the archive and remains queryable.

---

## 5. Reasoning Levels

Observations are stored at three levels, each with different reliability characteristics.

**Explicit** — directly extracted from a source. The PR description says "we're switching from nginx-ingress to envoy because of memory pressure at scale." The extracted Event records this decision with its rationale and the PR as the source artifact. The reasoning step is extraction only — no inference.

**Deductive** — logical combination of explicit facts. If Service A uses nginx-ingress (artifact-level fact from config) and we're migrating away from nginx-ingress (explicit, from PR), then Service A is in scope for the migration. The reasoning step is rule application over the graph. Reliability is bounded by the premises and the validity of the rule. A wrong premise propagates.

**Inductive** — generalized from multiple instances. Five incidents over six months all mention cache stampede as a contributing factor. The inductive observation: "cache stampede is a recurring failure mode." The reasoning step is pattern detection. Inductive observations are probabilistic — five correlated instances may not be causal, and the pattern may break with new data. They are stored as hypotheses, always cite the instances they generalize from, and are re-evaluated when new evidence arrives.

**Citation requirement.** Every deductive and inductive observation links to its source observations via `derived_from`. If a source observation is found to be wrong and archived, the derived observations it supported are flagged for re-evaluation. Without this, errors compound silently.

---

## 6. Consolidation

Consolidation is the asynchronous background process that produces deductive and inductive observations from stored explicit observations. It is batch, not online — it runs after ingestion, not during retrieval.

**Why batch.** Inductive reasoning requires enough instances to be meaningful. At ingestion time of the third cache-stampede incident, the pattern is not yet visible. At ingestion time of the fifth, it is. Consolidation runs over the accumulated corpus, not over individual events. It is also expensive (multiple LLM calls per batch) and should not be in the hot path of retrieval.

**Pipeline.**

1. **Sampling.** Select a slice of memory to consolidate: recent events attached to an active Project or Topic; observations with high surprisal (embedding distance from existing pattern — anomalies worth examining); observations not yet processed since last run.

2. **Load.** Pull the slice into an LLM context window. For a Project, this might be all Events and Notes from the past 30 days. For a Topic, all related Events across time.

3. **Deductive pass.** "Given these facts, what logical conclusions follow?" New observations are created at the `inferred` evidence level, linked back to their source observations.

4. **Inductive pass.** "What patterns appear across these instances?" New observations at the `synthesized` evidence level, citing the instances they generalize from. These are stored as hypotheses, not established facts.

5. **Contradiction pass.** "What conflicts here — between observations, or with prior organized knowledge?" Contradictions are surfaced as Events, not silently resolved.

6. **Store.** New observations written in `captured` state, with evidence type and `derived_from` links. They are not promoted to `organized` automatically.

**Promotion.** Inductive observations require an additional step before they are treated as established: either a human confirms, or an agent verifies the pattern against direct artifacts. This prevents synthesized hypotheses from silently becoming load-bearing facts.

**Failure modes.** Hallucinated patterns (instances that aren't actually related); compounding errors (deductive observation built on a faulty premise); confidently wrong inductive claims about people. The defenses: strict provenance, tentative status on synthesized output, contradiction detection as a first-class pass, no silent promotion.

---

## 7. Storage Architecture

Storage starts with SQLite. The interface is abstracted so the backend is replaceable without changing the retrieval or ingestion layers.

**Three layers in one store:**

- **Document store.** Every Portent object as a structured record: `id`, `type`, `title`, `content`, `lifecycle`, `evidence_type`, `recorded_at`, `metadata`.

- **Graph layer.** Objects as nodes, relationships as edges: `(subject_id, predicate, object_id, metadata)`. Predicates include `belongs_to`, `related_to`, `supersedes`, `derived_from`. Graph traversal via recursive CTEs — no graph database needed at this scale. An org knowledge graph of thousands of nodes is well within SQLite's range.

- **Semantic + full-text search.** Vector embeddings for semantic similarity (sqlite-vec extension). Full-text search via SQLite FTS5 for keyword matching. Both are necessary: semantic search finds "ingress controller migration" when queried as "how did we change routing"; FTS finds "nginx-ingress" when searched literally. Hybrid retrieval combines both result sets via reciprocal rank fusion (RRF).

**No tiktoken.** Go has no official tiktoken port. Token budgeting for consolidation batch sizing uses character-count approximation (1 token ≈ 4 characters for English text). This is sufficient for batch scheduling — it is not user-facing.

---

## 8. Ingestion

Ingestion is passive. The system extracts observations from ambient signals; humans do not need to write things down.

**Sources (examples):** Slack channels, GitHub issues and PRs, Jira tickets, meeting transcripts, deploy configs, infrastructure-as-code.

**Trust hierarchy.** Evidence type is assigned at ingestion based on source:
- Deploy configs and IaC → `artifact`
- Metrics and log events → `measured`
- Slack, GitHub issue comments, meeting transcripts → `reported`

Operational knowledge (log sinks, trace stores, environment inventory, service ownership) is most reliably extracted from IaC and deploy configs, not Slack threads. Slack threads are the source for decisions, discussions, and reported state changes.

**Pipeline.**

1. **Raw signal arrives** (webhook, poll, manual trigger).
2. **Entity resolution.** Extract entity references (service names, project names, people) and resolve them to existing graph nodes or create new ones.
3. **Observation extraction.** LLM pass over the signal to extract structured observations: what was said, what was decided, what state was reported. Assign evidence type based on source.
4. **Store and queue.** Write observations in `captured` state. Enqueue for consolidation if the relevant entity or topic has enough new signal to warrant a consolidation run.

**Channel → project mapping.** A key graph relation: Slack channels are related to Projects and Topics. This relation is extracted from channel descriptions, pinned messages, or explicit configuration. When Hector is @mentioned in a channel, it traverses this relation to load project context before responding.

---

## 9. Retrieval

Retrieval is not a single operation. It is a loop that composes graph traversal and semantic search.

**Context loading loop.**

1. **Entity resolution.** Identify the entities in the invocation context: the channel, the issue, the service name, the question topic.
2. **Graph traversal.** Starting from the resolved entities, traverse `belongs_to`, `related_to`, and `supersedes` to collect the relevant subgraph. This is the fast, structured path — it finds things that are explicitly connected.
3. **Semantic search.** Within the retrieved subgraph (or across the full corpus for broader queries), run hybrid retrieval (vector + FTS with RRF) against the query.
4. **Evidence filtering.** Rank results by evidence type. Artifact and measured evidence surfaces above reported. Synthesized hypotheses are marked as such in context.
5. **Inject.** The assembled context — typed objects with their evidence levels — is injected before the agent responds.

**Absence as a valid answer.** The retrieval layer must distinguish three states:
- *Found relevant observations* → return them with evidence.
- *Searched but found nothing* → "no documented information on this."
- *This domain has not been ingested* → "we haven't captured information from this source yet."

Conflating these produces false confidence. The coverage model tracks which sources and domains have been ingested so the system can distinguish the second and third cases.

**Memory vs RAG.** RAG over a document store retrieves documents and injects them. This system does the same, but the "documents" are typed, connected, provenance-tracked observations with supersession. The difference is not in the retrieval mechanism — it's in what is being retrieved. A RAG system confidently returns the wrong log location if the old runbook is still in the index. This system knows the old runbook is archived and superseded.

---

## 10. Evaluation

No existing benchmark tests organizational memory. The benchmarks that exist (LongMemEval, BEAM, LoCoMo) are built for personal conversational memory — flat streams of user preferences and biographical facts. The evaluation framework must be built alongside the system.

**Two fixture tiers.**

- **Unit fixtures** — 1–5 objects, one retrieval behavior in isolation. Fast, easy to diagnose on failure. Most test cases.
- **Integration fixtures** — 20–50 objects spanning a realistic project or topic. Tests consolidation output and cross-entity synthesis.

**Test case structure.**

```json
{
  "id": "TC-001",
  "category": "decision-retrieval",
  "fixture": [...objects to ingest, as typed Portent records...],
  "query": "Why did we choose Postgres over MySQL?",
  "rubric": [
    "mentions JSONB support as the rationale",
    "mentions MySQL as the rejected alternative",
    "does not assert general Postgres knowledge as org-specific"
  ],
  "expected_evidence_type": "artifact",
  "abstention": false
}
```

**Scoring.** Each rubric item scored 0 / 0.5 / 1.0 by an LLM judge. Final score is the average across nuggets. The judge prompt specifies the item being evaluated so scoring is compositional — a partially correct answer can score 0.5 on some nuggets and 1.0 on others.

**The hallucination nugget.** Every test case where the answer is org-specific should include: "does not assert general [domain] knowledge as org-specific." This is the failure mode that existing benchmarks do not test and that makes org memory systems untrustworthy.

**Provenance scoring.** An additional axis: did the system correctly qualify the evidence? A reported observation returned as established fact is a scoring failure even if the content is correct. This requires a `expected_evidence_type` field on each test case and a separate judge pass on the response's confidence calibration.

**Failure mode taxonomy:**
- **Hallucination** — general knowledge returned as org-specific fact.
- **False confidence** — hearsay asserted without qualification.
- **Omission** — relevant observation exists but was not retrieved.
- **False absence** — system says "no information" when information exists.
- **Staleness** — archived/superseded observation returned as current.
- **Provenance failure** — evidence type not surfaced or misrepresented.

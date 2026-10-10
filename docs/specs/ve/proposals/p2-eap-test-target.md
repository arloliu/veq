# P2 — The VE as the test target of the next-generation EAP

Status: draft (2026-10-10) — awaiting the owner

The VE exists so that host software can be built and verified without a production tool
([ve-architecture.md](../ve-architecture.md) §1).
So far the design checks one direction only: whether the VE behaves like the tool (the Evaluator, architecture §8).
This proposal designs the other: how a next-generation EAP is verified against VEs,
automatically and programmably from its first day, with tests that AI agents can write, run and improve.
The owner decisions behind it are recorded as V-31 in [ve-decisions.md](../ve-decisions.md).

It covers the VE's side and states what it requires of the EAP;
the EAP's own architecture is designed in the EAP's repository.

Terminology: `H->E` host to equipment, `E->H` equipment to host.
A *test environment* is one run's set of EAP, test gateways, VEs, upstream stubs and test clock.

---

## 1. Two directions of testing

| | VE validation | EAP verification (this proposal) |
|---|---|---|
| Question | does the VE behave like the tool? | does the EAP behave as its specification says, against tools that behave like the real ones? |
| Oracle | recorded traffic, fixtures with known outcomes | the EAP's specification, written into each scenario |
| Host side | Go host harness, historical replay (architecture §8.2) | the real EAP |
| Result | scores and promotion (architecture §8, §9) | a verdict per run and per expectation, and divergences fed back to the VE |

The two meet in one place:
a failing EAP test that turns out to be the VE's fault is a divergence,
and it joins the queue that offline replay already feeds (architecture §9).

The mandatory fixtures of architecture §8.3 keep their place in VE validation:
they stay promotion checks, run as they are today,
and a provisional Profile still passes those that apply ([ve-generation-service.md](../ve-generation-service.md) §1.2).
They are also written as scenarios of this proposal, which ask the other question:
(a) and (b) whether the real EAP handles the behaviour the fixture produces,
(c) and (d) the same through the FDC service in tier T2.

Test tiers, cheapest first, by what a run contains:

| Tier | Runs | Purpose |
|---|---|---|
| T0 | the EAP against in-process fakes | the EAP's unit tests; the EAP repository's business |
| T1 | the EAP, test gateways, VEs and upstream stubs | functional and fault scenarios: the bulk of this proposal |
| T2 | T1 plus the FDC service | scenarios whose oracle is the FDC service's output, such as fixtures (c) and (d) |
| T3 | T1 driven by recorded traffic | regression by replay; waits for the traffic log service (V-31) |

The clock a scenario runs on is chosen per scenario, not per tier (§3.3).
T2 runs on wall time while the gateway and the FDC service do.

## 2. Test topology

```
                         test runner
          ┌──────────────┬──────┴───────┬──────────────┐
          │ scenario     │ control      │ clock        │ upstream stubs
          ▼              ▼              ▼              ▼
   EAP ──NATS──► test gateway ──HSMS──► VE        MES and other stubs ◄──► EAP
   (next-gen)    (one per tool)         (one per tool)
```

- **The real EAP, unchanged.**
  It reaches each VE through a test instance of the equipment gateway, as it reaches a tool in production,
  so the gateway's own behaviour (session ID rewriting, its polling, its timeouts) is exercised too.
- **Separate from production.**
  Test gateways and their bus never share a bus or a cluster with production, for the reason of V-30.
- **One gateway instance per tool**, as in production (tracepack G5-200),
  so a scenario with a main tool and its load-port controller (V-18) runs two VEs and two gateway instances.
- **Upstream stubs** (MES and other systems the EAP talks to) sit behind the EAP's ports (ET-1);
  they belong to the EAP's repository, and the scenario drives and observes them through the test runner.
- **One environment per run**, created and destroyed by the runner,
  so runs do not share state and can run in parallel.

### 2.1 Observation points

The gateway rewrites, originates and drops traffic,
so what reaches the VE is not what the EAP sent.
A run therefore observes the equipment side at two boundaries:

- **The EAP's side**: the runner taps the test bus and records every message the EAP publishes or receives.
- **The tool's side**: the VE's Run Journal records every message on its HSMS connection.

Observations are correlated by run, tool, connection epoch, origin (the EAP, the gateway or the VE)
and forwarding outcome (forwarded, rewritten, dropped, timed out).
The forwarding outcome is read from the gateway where it reports one (§11),
otherwise inferred by matching the two sides and marked as inferred.
An observation gap, such as a tap that stopped or a message on one side with no match on the other, is reported;
an assertion that depends on the missing observations is `inconclusive`, never a pass.

## 3. The shared test clock

### 3.1 Why

In live-host mode the VE's scheduler follows wall time,
and with a continuously polling host there is no compression (architecture §10).
A process step of an hour then takes an hour to test.
With an EAP that takes an injectable clock (V-31, ET-2),
the EAP and the VE can run on one test clock, and time can pass faster than wall time.

### 3.2 Clock domains

Every timer in a test environment belongs to one clock domain:

| Timer | Owner | Domain |
|---|---|---|
| process progress, trajectories, scheduled Scenario steps, response delays | the VE's scheduler | test |
| T3 on the VE's own primaries (architecture §4.5) | the VE's scheduler | test |
| the EAP's own timeouts, retries and schedules | the EAP (ET-2) | test |
| the upstream stubs' timers | the stubs | test |
| HSMS T5–T8 and Linktest | go-secs, in the VE and in the gateway | wall |
| T3 on the primaries the gateway forwards; its polling for the FDC service; its task-deadline abort | the gateway | wall, unless the gateway takes the test clock (§3.4) |
| the timeouts of the EAP's bus client | the client library | wall |

The rule: **a timer and the delay it bounds belong to one domain.**
Where they do not, compression changes the outcome.
At ten times wall time, a VE reply delayed 20 s of test time arrives after 2 s of wall time,
inside a gateway T3 of 5 s that it would have missed at normal speed.
A scenario whose outcome depends on such a race runs on wall time.

The consequence, stated plainly:
while the gateway stays on wall time, compression shortens the waits that only test-domain timers bound,
such as process durations and the EAP's own timeouts;
transport faults and the gateway's timers are tested on wall time.

### 3.3 How time advances

Three modes, chosen per run through the Scenario's `clock mode` (architecture §3):

- **Wall**: the test clock is wall time.
  Every scenario can run so; it is the mode for cross-domain races and for T2.
- **Scaled**: the test clock runs at a declared *k* times wall time.
  Every participant reads one shared clock source, anchored at the run's start.
  The run declares *k* and a bound on how late, in wall time, a participant may deliver a timer.
  The runner marks the run `invalid(clock)` when either fails:
  a test-domain delay *d* bounded by a wall-domain timer *T* falls between *T* and *k*·*T*,
  so that compression inverts its outcome;
  or a participant delivers a timer later than the bound, so that *k* times its lag distorts test time.
- **Stepped**: the test clock jumps to the next deadline whenever the whole environment is quiescent:
  - no participant has runnable work, meaning work that can proceed without time passing;
  - every message is accounted for:
    delivered, or dropped by a declared fault, and none in flight on the bus or in the gateway's queues;
  - the jump goes to the earliest deadline any participant reports,
    including the timeouts of open transactions:
    a transaction awaiting a reply that will not come waits on a deadline, which is not runnable work.

  Every intermediary takes part, the test gateway included,
  so stepped time needs the gateway to take the test clock and report its work (§11).
  A wall-time watchdog marks the run `invalid(clock)` when quiescence is not reached within its bound.
  This is a second jump rule beside the VE's idle policy of architecture §10, which never jumps across an open T3;
  it does not replace that policy for a host on wall time.

Recommended: scaled time first, because it needs nothing from the participants beyond reading the clock;
stepped time once the EAP, the VE and the test gateway report their work,
because it removes waiting entirely and is the mode in which an hour-long step takes seconds.
The coordination protocol is left to implementation (§12).

`timing scale` in the Instance Overlay (architecture §3) is a different control:
it shortens the tool's own durations, an authored change labelled as such,
whereas the test clock keeps every duration and compresses only the waiting.

### 3.4 The test gateway

The gateway runs HSMS toward the VE and its own application timers on wall time.
In a compressed run its polling becomes sparse in test time, and a deadline-based abort fires late.
If the test gateway takes the test clock, including its T3, which is a request to the gateway's project (§11),
its timers join the test domain and stepped time becomes possible.
Until then, scenarios that depend on the gateway's timers run on wall time.

### 3.5 Reproducibility

A compressed run is fast and recordable, not bit-identical from run to run:
in live mode the VE's admission order may differ from wire order (architecture §10), and so may the EAP's.
Each component's part of a failed run can be replayed offline and deterministically from what the run kept:

- **The VE's Reproduction Bundle**, replayed in offline/core mode (architecture §10).
- **The EAP's replay tape and decision log** (ET-3, ET-4), replayed as the EAP's repository provides.
- **The run manifest**: the builds and configuration of the EAP, the test gateway, the stubs and the runner;
  the scenario and the versions of the requirements it cites;
  the VE's launch inputs (Profile version, tool delta, Instance Overlay, runtime version, seed, initial checkpoint);
  and the clock mode, *k* and anchors.

## 4. The VE control plane

The runner drives each VE through a control API:

- **Load**: a Profile version, tool delta, Instance Overlay, seed and initial checkpoint (architecture §3).
- **Describe**: the loaded Profile's self-description:
  identifiers, events and their reports, remote commands and their parameters, machines and their states,
  the fault overlays and quirks it can exhibit, and each rule's provenance stage (architecture §11).
  Scenario authors, human or agent, build from it instead of guessing.
- **Stimulate**: apply a Scenario step or a fault overlay; the answer is the authority rule's outcome
  (`performed`, `accepted-pending`, `rejected` or `stalled`, architecture §3.2).
  Guards are evaluated here, at run time; a stimulus never sets a guard directly.
- **Observe**: Instance State, open transactions, and the Run Journal from a position onward.
- **Checkpoint and reset**: take a checkpoint, restore one, or apply a Scenario restart (architecture §3.1).
- **Clock**: join a test clock and report runnable work and the next deadline (§3.3).

The API is the same for a human, a Go test and an AI agent.
Its transport and schema are left to implementation.

## 5. Scenarios

### 5.1 Form

A scenario is a declarative file, validated by a published schema (V-31).
It holds:

- **requirements**: the EAP requirements it checks, each with its version (§7);
- **setup**: the tools by role, each with a Profile version and an initial state; the upstream stubs; the clock mode;
- **steps**: stimuli on the equipment side (operator actions, material, alarms, fault overlays),
  on the upstream side (a stub's message), or waits for a condition;
- **expectations**, read from the observation points (§2.1), the EAP's decision log (ET-3),
  the upstream stubs, and the VE's outcome for a stimulus (§4).

Expectations need these capabilities, settled now; their grammar is left to implementation:

- **Presence within a bound**: a message, decision or outcome occurs within a test-time limit.
- **Bounded absence**: something does not occur within a limit.
- **Invariants**: a condition holds throughout a window, such as the VE's processing state being unchanged.
- **Windows armed before the stimulus**, so a reaction faster than the step that looks for it is still seen.
- **Correlation**: the reply to a given primary, across the gateway's rewriting.
- **Ordering and counts**: one event before another; exactly *n* of a message.
- **Parallel branches** per tool, joined before the next step.

Fixture (a) as a sketch, with roles in place of identifiers:

```yaml
scenario: start-rejected-then-retried
requirements: [<eap-requirement>@<version>]
setup:
  tools:
    - role: main-tool
      profile: <tool-type>@<version>
      state: idle-remote
  clock: scaled
steps:
  - ve: main-tool applies overlay cannot-perform-now
  - watch:
      id: not-running
      for: 10s
      never: eap-decision lot-running
      invariant: main-tool processing-state unchanged
  - upstream: mes sends start-lot
  - expect:
      within: 10s
      eap-sends: S2F41 START to main-tool
      tool-receives: S2F41 START
      ve-outcome: rejected
      upstream-receives: mes start-failed
  - join: not-running
  - ve: main-tool clears overlay cannot-perform-now
  - upstream: mes sends start-lot
  - expect:
      within: 10s
      ve-outcome: performed
      eap-decision: lot-running
```

The guard fails because a fault overlay the Profile declares is applied, not because the scenario sets it;
a guard can equally be reached through a state transition the Profile supports.
The vocabulary of steps and expectations is small and closed.
A Go SDK drives the same control plane for what the file cannot express (V-31).

### 5.2 Validation before running

A scenario is checked without running it,
against the schema and against the self-description of the effective Profile, tool delta and Instance Overlay:
every identifier, state, overlay and role exists, and every capability it uses is supported.
Guards are not checked statically:
they depend on run-time state and on what the EAP does meanwhile,
and an expected `rejected` or `stalled` outcome is a legitimate test.
A scenario that fails here costs no environment.

## 6. Results and triage

Each run produces a machine-readable report, which starts with a run verdict:

- **valid**;
- **`invalid(infra)`**: the VE journaled `overload` or `possible-cross-epoch-write` (architecture §4.5, §4.6);
- **`invalid(clock)`**: a scaled run broke its bounds, or a stepped run hit its watchdog (§3.3);
- **`invalid(environment)`**: a component failed to start, or an observation point failed (§2.1).

An invalid run's passes count for nothing; its infrastructure diagnostics are part of the report.
For a valid run, the report gives per expectation pass, fail or `inconclusive`;
on failure, the expected and observed values,
the correlated observations and the EAP's decision log around it,
and the provenance stage and any measured validation of each VE rule the failing path used.

A failure in a valid run falls into one of four classes:

| Class | Typical sign | Goes to |
|---|---|---|
| EAP defect | the observations show the VE acted as recorded traffic or measured validation says the tool does, and the EAP's decision contradicts the scenario | the EAP's developers |
| Scenario defect | the expectation contradicts the cited requirement, or the scenario passes validation yet tests nothing | the scenario's author |
| VE doubt | the VE's behaviour on the failing path has no measured validation, or contradicts it | a divergence in the VE's queue (architecture §9), confirmed on a real tool before the VE changes |
| Unresolved | the evidence points to none of these, or to more than one | a person |

Provenance stages are context, never the reason for a class: a stage is not a fidelity claim (architecture §11).
The class is a suggestion from the report; a person or an agent confirms it.

## 7. The agent loop

Agents run inside the fab, within the boundary of [ve-generation-service.md](../ve-generation-service.md) §3:
Profiles, journals and reports are log-derived and never leave it ([ve-decisions.md](../ve-decisions.md) V-32).
Agents hosted outside the fab are not part of this design.

An AI agent improves tests in a loop that needs no person until a result is disputed:

1. **Generate** a scenario from the Profile's self-description and a cited EAP requirement,
   and, where useful, from the vendor manual's pages.
2. **Validate** it (§5.2), and repair what fails.
3. **Run** it in tier T1.
4. **Read** the report (§6), and revise the scenario or file the finding by its class.
5. **Aim** the next scenario at what no scenario reaches yet:
   the runner reports coverage of Profile states, events, remote commands, fault overlays and quirks.
6. **Check strength** by mutation (below).

The oracle is guarded against the loop that writes it:

- **Expectations follow requirements.**
  Each scenario cites the requirements it checks, with their versions.
  Automatic repair may change a scenario's setup and steps, never its expectations;
  a change to an expectation is a separate change that cites a requirement change and is approved by a person.
- **Failed runs are kept**, with their reports and manifests, whatever later runs show.
- **Mutation is targeted.**
  Each mutant, a deliberately broken EAP build or VE Profile, names the behaviour it breaks
  and the expectations that must catch it, as the Evaluator's negative controls do for the VE (architecture §8.1).
  A mutant counts as caught only by a valid run that fails at those expectations;
  an invalid run, or a mutant Profile that fails its own validation, catches nothing.

The loop is fast only because T1 runs compressed where the clock domains allow it, and validation is static.

## 8. Requirements on the next-generation EAP

Requirements only; how the EAP meets them is its repository's design.

- **ET-1 Ports.**
  Every external system (the equipment gateway, MES, other upstream systems)
  sits behind an interface the tests can point at a test instance or a stub.
- **ET-2 Injectable clock.**
  Every timer, timeout and schedule of the EAP's own reads one clock interface,
  which a test sets to the shared test clock (§3).
  The bus client library's timeouts stay on wall time (§3.2).
- **ET-3 Decision log.**
  The EAP records each decision it takes
  (what it decided, from which inputs, which messages it sent because of it)
  as structured events with identifiers that tie them to the triggering messages.
- **ET-4 Replay tape.**
  The EAP records, in the order it admits them, every external outcome its decisions depend on:
  messages received, timer firings, send results, connection loss and reconnection, and stub replies.
  Replay injects the tape and recomputes the decisions, which are compared with the decision log, never injected;
  this is the shape of the VE's offline mode (architecture §10).
  It requires no hidden wall-clock reads, unseeded randomness or unordered iteration in decisions.
- **ET-5 Test hooks.**
  The EAP can be started in a known state, reports when it is ready,
  and, for stepped time, reports runnable work and its next deadline (§3.3).

## 9. Day 1: provisional Profiles

A new tool type has a Profile from its manual alone before any log arrives (architecture §11, *manual-derived*),
enough for functional scenarios.
Every result names the provenance stage of the rules it relied on,
so a pass on a manual-derived rule reads as "consistent with the manual", not "consistent with the tool".
As logs and divergences improve the Profile, the same scenarios run again on the new version.

## 10. Changes if accepted

- [ve-architecture.md](../ve-architecture.md):
  - §1 states EAP verification as the VE's purpose beside VE validation,
    and goal 3 qualifies "host-facing timing is wall time in live mode" for a host on the test clock;
  - §2 adds the test runner, test gateways and the test clock to the overview;
  - §3 adds the control plane's operations to the Runtime,
    and defines `clock mode` (wall, scaled, stepped) beside `timing scale`;
  - §8.2 and §8.3 add EAP scenarios as a second path for every mandatory fixture,
    and keep the existing paths as the promotion checks;
  - §9 adds triaged EAP test failures to the divergence sources;
  - §10 makes live-host scheduler time follow the host-facing clock, wall or test,
    adds the stepped mode's quiescence rule beside the idle policy,
    and qualifies "no compression with a polling host" by V-31.
- A new living spec for EAP verification holds §2 to §9 of this proposal when it closes.

## 11. Open questions

| Question | For |
|---|---|
| Can the test gateway take the test clock, its T3 included, report runnable work and deadlines, and report each message's forwarding outcome? | gateway owners |
| Which upstream systems the next-generation EAP talks to, and who builds their stubs | EAP team |
| Where scenarios live: the EAP's repository, veq, or a shared one | owner |
| Who approves a change to a scenario's expectations | owner |
| The scenario grammar and the control plane's transport | implementation, after acceptance |
| The scaled mode's usable *k* and timer-lag bound | measured |
| How many VEs one test environment runs, and how many environments run in parallel | measured |

## 12. Left to implementation

The clock coordination protocol;
the control plane's transport and schema;
the scenario grammar and schema;
the report format;
the runner and its environment provisioning.

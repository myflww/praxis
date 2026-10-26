# Praxis

**Praxis · 知行** · Skill ID: `praxis`

Help an agent carry authorized work through to a verifiable result, and help you check your understanding when you ask. Everyday collaboration silently considers four kinds of information. Understanding checks use real code or a supplied design: the agent asks, waits for your answer, then explains any gaps with evidence.

[中文](README.md) · [Usage guide](docs/USAGE.md) · [Provenance and independent implementation](docs/PROVENANCE.md) · [Feature comparison and validation](docs/COMPARISON.md)

## What it does

| Capability | In practice |
| --- | --- |
| Reuse shared knowledge | Carry forward agreed goals, scope, and decisions; complete authorized work without repeatedly confirming them. |
| Identify what only the user knows | Ask about personal preferences or real-world constraints only when the answer materially changes the result. |
| Discover facts the agent can find | Read relevant files and implementations, surface omissions, challenge weak premises, and explain evidence and trade-offs. |
| Make shared uncertainty testable | Reduce a consequential assumption to a small question with clear observations; execution still follows the existing authorization. |
| Check understanding on request | Choose a mechanism from real code or a design, ask for a prediction, explanation, or boundary analysis, then respond to the reasoning. |

The four quadrants guide internal judgment. They do not require four recurring headings. Simple requests stay simple: a request to explain code receives an explanation, and a request to fix an issue proceeds toward a fix. A quiz is never a prerequisite for help.

## Why it helps

- **Fewer repeated clarifications:** reuse settled goals and decisions; ask only about material gaps.
- **Reviewable understanding:** ground questions and feedback in the current implementation and its failure paths.
- **Work and practice stay separate:** complete ordinary work directly; start a knowledge check when requested.
- **Small, inspectable components:** load references when needed and ship native picker source with verifiable artifact hashes.


## Useful situations

- After delegating development, checking whether you can explain the key behavior and trade-offs.
- Taking over a module and examining your understanding of configuration precedence, cancellation, or cache isolation.
- Reviewing a design and separating established facts from inferences and missing evidence.
- Everyday collaboration where the agent should discover available context and avoid repeating settled questions.

An understanding check normally asks one core question and waits for your answer before giving feedback. You can choose the language, difficulty, question count, or batch format, or switch to a direct explanation at any time. It can reveal gaps in current understanding; it does not promise complete mastery or a guaranteed learning outcome.

Example prompts:

> Use $praxis to complete this change within the agreed scope and verify the result.
>
> Check whether I understand this module's cancellation behavior. Ask one question at a time.
>
> Limit this to src/cache. Give me three questions in English at once, then wait until I answer them all.
>
> Explain the configuration loading order directly. No quiz.

## Installation

Repository: [myflww/praxis](https://github.com/myflww/praxis).

Copy the **entire `skills/praxis` directory** into your host's skill directory. A common location is `~/.agents/skills/`; existing installations may use `~/.codex/skills/`. Follow your host's actual configuration. Install this one Skill directory rather than the whole repository.

For a fresh installation, run from the repository root:

```sh
mkdir -p "$HOME/.agents/skills"
cp -R skills/praxis "$HOME/.agents/skills/"
```

If a Skill with the same ID already exists, preserve a backup and any personal edits before replacing the entire directory. Avoid mixing files from different versions. Keep the bundled `LICENSE`, `NOTICE`, and `third_party/`; copying only `SKILL.md` is insufficient. Reload skills or start a new conversation as required by your host.

## Native reading-start selector

When you specify a file or feature, the agent reads that scope directly. Without a scope, the bundled CLI can select a reading starting point. It neither writes questions nor judges whether a file makes useful quiz material. The agent tries at most three candidate starting points, then explains the missing context instead of scanning indefinitely.

Executable names under the Skill's `bin/` directory:

| System | amd64 | arm64 |
| --- | --- | --- |
| macOS | `pick-topic-darwin-amd64` | `pick-topic-darwin-arm64` |
| Linux | `pick-topic-linux-amd64` | `pick-topic-linux-arm64` |
| Windows | `pick-topic-windows-amd64.exe` | `pick-topic-windows-arm64.exe` |

These are native build targets. Running a compiled binary does not require Python or the Go development toolchain. See [feature comparison and validation](docs/COMPARISON.md) for actual builds, test results, and differences; cross-compilation alone is not evidence of execution on every target platform.

The interface is `bin/pick-topic-{darwin,linux,windows}-{amd64,arm64}[.exe] [REPO]`, with repeatable `--exclude-path PATH` and `--seed INT` for reproducible selection. Omitting `REPO` uses the current directory.

```sh
./skills/praxis/bin/pick-topic-darwin-arm64 path/to/repo --exclude-path private --exclude-path .env --seed 23
```

The selector runs locally without networking. On success it prints one relative path, never file contents, and does not edit the repository. With Git available inside a worktree, it uses Git candidate discovery and ignore semantics. Outside a Git worktree, without Git, or when the Git query fails, it silently falls back to local traversal; project ignore rules are not guaranteed in that mode.

**File filtering is not secret filtering.** Names such as `.env`, `.pem`, and `.key` can still be selected. Before reading selected contents, the agent must assess sensitivity and exclude unsuitable material. Explicitly exclude known sensitive paths. Valid file symlinks whose targets remain within the selection root may be eligible; links escaping that root are rejected. Subsequent agent reads and model processing follow the host's configuration and are outside the CLI's local-execution guarantee.

## Evidence and authorization

Questions start with actual code or a design supplied by the user. Feedback identifies the correct causal reasoning, the point of divergence, and the evidence supporting a correction. Missing evidence is not filled with an invented answer. Changes to the material or a challenge from the user call for renewed verification. Understanding points remain in the current conversation by default, without persistent memory.

“Quiz me” authorizes reading needed for the check. It does not automatically authorize tests, services, file edits, commits, or publication. Reading source and answering a limited set of questions cannot establish implementation correctness or comprehensive mastery.

## Provenance and license

This project learns from the user-facing capability of source-based questions and understanding feedback demonstrated by GrillMe. Its wording and implementation are independently designed against an explicit feature checklist. The distribution does not bundle GrillMe's Skill text, implementation code, icons, or binaries. The selector is independently written with the Go standard library and no third-party Go packages. See [PROVENANCE](docs/PROVENANCE.md) for the research method, known limits, and source acknowledgment.

Project-authored content uses [Apache-2.0](LICENSE); see [NOTICE](NOTICE) for copyright and source acknowledgment. Commercial use, modification, and redistribution are permitted subject to the license, including preservation of applicable license, copyright, and attribution notices and notices of changed files. The license does not require advertising-style credit in every user interface or disclosure of derivative source code. Relevant Go toolchain and runtime licenses are listed separately under the Skill's [third_party/](skills/praxis/third_party/).

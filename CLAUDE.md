# veq — Claude Code Configuration

## Identity

**veq** (`github.com/arloliu/veq`) builds Virtual Equipment:
passive-side simulators of semiconductor tools, derived from recorded host–equipment traffic and vendor GEM manuals.

## Working Principles

- **No sensitive data in the repository.**
  Real inputs stay in the gitignored `data/` tree; see `100-overview.md`.
- **Surface uncertainty before coding.**
  If multiple interpretations exist, present them; if unclear, ask.
- **Minimum change that solves the problem.**
  No speculative features or unasked-for flexibility.
- **Don't guess — verify.**
  Write a small test; don't assume behaviour of tools or EAPs without evidence.

## Git

Never add `Co-Authored-By` or any attribution trailer.

## Rules

@AGENTS.md

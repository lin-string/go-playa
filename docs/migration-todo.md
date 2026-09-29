# Migration status

English | [简体中文](migration-todo.zh-CN.md)

The tracked Playa migration is complete.

This file remains as a stable target for historical links. It is no longer an
active checkbox backlog. Current maintenance uses these sources of truth:

- [`compat/upstream.toml`](../compat/upstream.toml) identifies the pinned Playa
  oracle. Documentation does not duplicate that version.
- [`compat/manifest.toml`](../compat/manifest.toml) defines the compared public
  API sections. Every listed section is required.
- [`migration.md`](migration.md) defines scope, interface adaptation, laziness,
  ownership, lifecycle, and intentional differences.
- [`compatibility.md`](compatibility.md) defines the corpus comparison and
  release acceptance workflow.
- [`engineering.md`](engineering.md) defines the gates for future changes.

The completed scope includes PDF parsing and recovery, encryption, page and
object traversal, content interpretation, layout, fonts and CMaps, images,
annotations, destinations and actions, outlines, forms, tagged-PDF structure,
the inspection CLI, bounded concurrency, and final public domain-package
ownership.

New producer-specific failures are defects or corpus-hardening work, not an
unfinished migration phase. A newly accepted Playa API must first be mapped in
the compatibility manifest and migration contract, then implemented with the
required behavior, ownership, and oracle tests.

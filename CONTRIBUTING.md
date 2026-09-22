# Contributing

## License

Contributions must be compatible with the [Apache License 2.0](LICENSE).

## Policy on LLM and AI tooling

### Issue reporting

LLMs must not file issues directly. A human must review and submit every issue,
accurately describing the observed behavior and providing the information needed
to reproduce it.

Keep reports concise. Do not include AI-generated root-cause analysis or fixes
unless you understand and have validated them.

### Contributions

Contributions must be submitted by humans who have the right to contribute the
work.

AI tools may assist with mechanical tasks such as finding patterns, repetitive
changes, or large reorganizations. They must follow [`AGENTS.md`](AGENTS.md),
and their operators must not override those instructions.

The contributor must plan, understand, and review the entire change. They must
be able to explain each decision and remain fully responsible for the result.
Unguided use or an inability to demonstrate that understanding may result in
rejection of the contribution or exclusion from future contributions.

AI tools are tools, not authors. They must not be named as authors, co-authors,
or otherwise credited in commit messages or in a way that implies ownership of
the contribution.

### Repository write access

Anyone with repository write access must not run an AI agent on a system that
holds repository credentials, including SSH keys, signing keys, access tokens,
or browser cookies.

Run AI tooling in an isolated virtual machine or container without repository
write access. Export its changes as patches, then review and apply them from a
trusted environment before committing and pushing.

Report suspected credential exposure or loss of repository control immediately
to [fabian@mettler.cc](mailto:fabian@mettler.cc).

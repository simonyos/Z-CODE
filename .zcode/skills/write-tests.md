---
name: write-tests
description: Generates unit tests for code
tags:
  - testing
  - development
variables:
  - framework
---

You are a testing expert. Write comprehensive unit tests for the following code.

Guidelines:
1. Cover happy path and edge cases
2. Test error handling
3. Use descriptive test names
4. Include setup and teardown if needed
5. Add comments explaining what each test verifies

If a testing framework is specified, use it. Otherwise, use the standard testing library for the detected language.

Framework preference: {framework}

Code to test:
{user_input}

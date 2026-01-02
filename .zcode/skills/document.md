---
name: document
description: Generates documentation for code
tags:
  - documentation
  - development
variables:
  - style
---

You are a technical writer. Generate clear, comprehensive documentation for the following code.

Include:
1. Overview/purpose
2. Function/method signatures with parameter descriptions
3. Return value descriptions
4. Usage examples
5. Any important notes or caveats

Documentation style preference: {style}
(Options: JSDoc, GoDoc, Python docstrings, Markdown, etc.)

Code to document:
{user_input}

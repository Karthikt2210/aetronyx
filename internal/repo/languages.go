// Package repo provides tree-sitter query strings per language.
package repo

// goImportQuery extracts import paths from Go source files.
var goImportQuery = `
(import_spec path: (interpreted_string_literal) @import)
`

// goDefQuery extracts function, method, and type definitions from Go files.
var goDefQuery = `
(function_declaration name: (identifier) @func)
(method_declaration name: (field_identifier) @func)
(type_spec name: (type_identifier) @type)
`

// tsImportQuery extracts import paths from TypeScript source files.
var tsImportQuery = `
(import_statement source: (string) @import)
`

// tsDefQuery extracts class, function, interface, and type definitions from TypeScript.
var tsDefQuery = `
(class_declaration name: (type_identifier) @class)
(function_declaration name: (identifier) @func)
(interface_declaration name: (type_identifier) @iface)
(type_alias_declaration name: (type_identifier) @type)
`

// jsImportQuery extracts import paths from JavaScript source files.
var jsImportQuery = `
(import_statement source: (string) @import)
`

// jsDefQuery extracts function and class definitions from JavaScript.
var jsDefQuery = `
(function_declaration name: (identifier) @func)
(class_declaration name: (identifier) @class)
`

// pyImportQuery extracts module paths from Python import statements.
var pyImportQuery = `
(import_statement name: (dotted_name) @import)
(import_from_statement module_name: (dotted_name) @import)
(import_from_statement module_name: (relative_import) @import)
`

// pyDefQuery extracts function and class definitions from Python.
var pyDefQuery = `
(function_definition name: (identifier) @func)
(class_definition name: (identifier) @class)
`

// rustImportQuery extracts use-declaration paths from Rust source files.
var rustImportQuery = `
(use_declaration argument: (_) @import)
`

// rustDefQuery extracts function, struct, trait, and impl definitions from Rust.
var rustDefQuery = `
(function_item name: (identifier) @func)
(struct_item name: (type_identifier) @type)
(trait_item name: (type_identifier) @iface)
(impl_item type: (type_identifier) @class)
`

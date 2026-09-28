#!/usr/bin/env node
//
// Validate the data files against their schemas, with no dependency: the two
// schemas here use a small subset of JSON Schema (type, required, properties,
// additionalProperties, enum, const, pattern, oneOf, $ref to #/$defs, minimum,
// minItems, minLength, propertyNames, minProperties). Anything outside that
// subset is an error here, so a schema cannot quietly grow past what is checked.
//
// The Go side checks the manifest again at startup (services.ValidateManifest),
// with the rules a schema cannot state. This is the editor-time half.
//
//   node scripts/validate-schemas.mjs
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import path from "node:path";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const read = (rel) => JSON.parse(readFileSync(path.join(root, rel), "utf8"));

const PAIRS = [
  ["design/tokens.json", "design/tokens.schema.json"],
  ["data/launcher.json", "design/launcher.schema.json"],
];

const KNOWN = new Set([
  "$schema",
  "$id",
  "title",
  "description",
  "$defs",
  "type",
  "required",
  "properties",
  "additionalProperties",
  "enum",
  "const",
  "pattern",
  "oneOf",
  "allOf",
  "$ref",
  "minimum",
  "exclusiveMinimum",
  "exclusiveMaximum",
  "maximum",
  "minItems",
  "maxItems",
  "items",
  "minLength",
  "propertyNames",
  "minProperties",
]);

function typeOf(v) {
  if (v === null) return "null";
  if (Array.isArray(v)) return "array";
  if (typeof v === "number") return Number.isInteger(v) ? "integer" : "number";
  return typeof v;
}

function matchesType(v, t) {
  const actual = typeOf(v);
  if (t === "number") return actual === "number" || actual === "integer";
  return actual === t;
}

function resolve(schema, rootSchema) {
  if (!schema.$ref) return schema;
  if (!schema.$ref.startsWith("#/"))
    throw new Error(`unsupported $ref ${schema.$ref}`);
  let node = rootSchema;
  for (const part of schema.$ref.slice(2).split("/")) node = node[part];
  if (!node) throw new Error(`dangling $ref ${schema.$ref}`);
  return node;
}

function validate(value, schema, rootSchema, at, errors) {
  for (const key of Object.keys(schema)) {
    if (!KNOWN.has(key))
      errors.push(`${at}: schema uses unsupported keyword "${key}"`);
  }
  schema = resolve(schema, rootSchema);
  if (schema.allOf)
    for (const s of schema.allOf) validate(value, s, rootSchema, at, errors);
  if (schema.oneOf) {
    const passing = schema.oneOf.filter((s) => {
      const e = [];
      validate(value, s, rootSchema, at, e);
      return e.length === 0;
    });
    if (passing.length !== 1)
      errors.push(`${at}: matches ${passing.length} of oneOf, want 1`);
  }
  if (
    schema.const !== undefined &&
    JSON.stringify(value) !== JSON.stringify(schema.const)
  ) {
    errors.push(`${at}: must be ${JSON.stringify(schema.const)}`);
  }
  if (schema.enum && !schema.enum.includes(value)) {
    errors.push(
      `${at}: ${JSON.stringify(value)} is not one of ${JSON.stringify(schema.enum)}`,
    );
  }
  if (schema.type) {
    const types = Array.isArray(schema.type) ? schema.type : [schema.type];
    if (!types.some((t) => matchesType(value, t))) {
      errors.push(`${at}: is ${typeOf(value)}, want ${types.join("|")}`);
      return;
    }
  }
  if (typeof value === "number") {
    if (schema.minimum !== undefined && value < schema.minimum)
      errors.push(`${at}: below minimum`);
    if (schema.maximum !== undefined && value > schema.maximum)
      errors.push(`${at}: above maximum`);
    if (
      schema.exclusiveMinimum !== undefined &&
      value <= schema.exclusiveMinimum
    )
      errors.push(`${at}: not above ${schema.exclusiveMinimum}`);
    if (
      schema.exclusiveMaximum !== undefined &&
      value >= schema.exclusiveMaximum
    )
      errors.push(`${at}: not below ${schema.exclusiveMaximum}`);
  }
  if (typeof value === "string") {
    if (schema.minLength !== undefined && value.length < schema.minLength)
      errors.push(`${at}: shorter than ${schema.minLength}`);
    if (schema.pattern && !new RegExp(schema.pattern).test(value))
      errors.push(
        `${at}: ${JSON.stringify(value)} does not match ${schema.pattern}`,
      );
  }
  if (Array.isArray(value)) {
    if (schema.minItems !== undefined && value.length < schema.minItems)
      errors.push(`${at}: fewer than ${schema.minItems} items`);
    if (schema.maxItems !== undefined && value.length > schema.maxItems)
      errors.push(`${at}: more than ${schema.maxItems} items`);
    if (schema.items)
      value.forEach((v, i) =>
        validate(v, schema.items, rootSchema, `${at}[${i}]`, errors),
      );
  }
  if (value && typeof value === "object" && !Array.isArray(value)) {
    const keys = Object.keys(value);
    if (
      schema.minProperties !== undefined &&
      keys.length < schema.minProperties
    )
      errors.push(`${at}: fewer than ${schema.minProperties} properties`);
    for (const req of schema.required ?? []) {
      if (!(req in value)) errors.push(`${at}: missing required "${req}"`);
    }
    for (const key of keys) {
      if (schema.propertyNames)
        validate(
          key,
          schema.propertyNames,
          rootSchema,
          `${at}.${key} (name)`,
          errors,
        );
      const prop = schema.properties?.[key];
      if (prop) validate(value[key], prop, rootSchema, `${at}.${key}`, errors);
      else if (schema.additionalProperties === false)
        errors.push(`${at}: unknown property "${key}"`);
      else if (
        schema.additionalProperties &&
        typeof schema.additionalProperties === "object"
      )
        validate(
          value[key],
          schema.additionalProperties,
          rootSchema,
          `${at}.${key}`,
          errors,
        );
    }
  }
}

let failed = false;
for (const [dataPath, schemaPath] of PAIRS) {
  const errors = [];
  const schema = read(schemaPath);
  validate(read(dataPath), schema, schema, dataPath, errors);
  if (errors.length) {
    failed = true;
    console.error(
      `${dataPath}: ${errors.length} problem(s) against ${schemaPath}`,
    );
    for (const e of errors) console.error(`  ${e}`);
  } else {
    console.log(`${dataPath}: valid against ${schemaPath}`);
  }
}
process.exit(failed ? 1 : 0);

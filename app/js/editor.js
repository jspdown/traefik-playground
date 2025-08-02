import { EditorState } from "@codemirror/state";
import { EditorView, keymap, lineNumbers } from "@codemirror/view";
import { defaultKeymap } from "@codemirror/commands";
import {yaml} from "@codemirror/lang-yaml"
import {HighlightStyle, syntaxHighlighting} from "@codemirror/language"
import {linter, lintGutter} from "@codemirror/lint";
import { tags as t } from "@lezer/highlight";
import {Kind, parseWithPointers} from "@stoplight/yaml";
import AJV from "ajv"
import betterAjvErrors from "better-ajv-errors";

import traefikSchema from "./traefik-v3.schema.json"

export function enhanceEditor(originalEditor) {
    const theme = EditorView.theme({
        "&": {
            backgroundColor: "#1d1f21",
            color: "#FFFFFF",
        },
        ".cm-gutters": {
            backgroundColor: "#1d1f21",
            color: "#646464"
        },
        ".cm-content": {
            caretColor: "#FFFFFF",
        },
        ".cm-activeLine": {
            backgroundColor: "#4c566a29"
        },
        ".cm-activeLineGutter": {
            backgroundColor: "#4c566a29",
            color: "#d8dee9"
        },
        "&.cm-focused .cm-selectionBackground, & .cm-line::selection, & .cm-selectionLayer .cm-selectionBackground, .cm-content ::selection": {
            background: "rgba(36,161,193,0.2) !important"
        },
        "& .cm-selectionMatch": {
            backgroundColor: "rgb(36, 161, 193, 0.2)",
        }
    }, { dark: true })

    const style = syntaxHighlighting(HighlightStyle.define([
        { tag: t.keyword, color: '#5e81ac' },
        { tag: [t.name, t.deleted, t.character, t.propertyName, t.macroName], color: '#88c0d0' },
        { tag: [t.variableName], color: '#8fbcbb' },
        { tag: [t.function(t.variableName)], color: '#8fbcbb' },
        { tag: [t.labelName], color: '#81a1c1' },
        { tag: [t.color, t.constant(t.name), t.standard(t.name)], color: '#5e81ac' },
        { tag: [t.definition(t.name), t.separator], color: '#a3be8c' },
        { tag: [t.brace], color: '#8fbcbb' },
        { tag: [t.annotation], color: '#0151d3' },
        { tag: [t.number, t.changed, t.annotation, t.modifier, t.self, t.namespace], color: '#b48ead' },
        { tag: [t.typeName, t.className], color: '#ebcb8b' },
        { tag: [t.operator, t.operatorKeyword], color: '#a3be8c' },
        { tag: [t.tagName], color: '#b48ead' },
        { tag: [t.squareBracket], color: '#bf616a' },
        { tag: [t.angleBracket], color: '#d08770' },
        { tag: [t.attributeName], color: '#ebcb8b' },
        { tag: [t.regexp], color: '#5e81ac' },
        { tag: [t.quote], color: '#b48ead' },
        { tag: [t.string], color: '#a3be8c' },
        {
            tag: t.link,
            color: '#a3be8c',
            textDecoration: 'underline',
            textUnderlinePosition: 'under',
        },
        { tag: [t.url, t.escape, t.special(t.string)], color: '#8fbcbb' },
        { tag: [t.meta], color: '#88c0d0' },
        { tag: [t.monospace], color: '#d8dee9', fontStyle: 'italic' },
        { tag: [t.comment], color: '#4c566a', fontStyle: 'italic' },
        { tag: t.strong, fontWeight: 'bold', color: '#5e81ac' },
        { tag: t.emphasis, fontStyle: 'italic', color: '#5e81ac' },
        { tag: t.strikethrough, textDecoration: 'line-through' },
        { tag: t.heading, fontWeight: 'bold', color: '#5e81ac' },
        { tag: t.special(t.heading1), fontWeight: 'bold', color: '#5e81ac' },
        { tag: t.heading1, fontWeight: 'bold', color: '#5e81ac' },
        {
            tag: [t.heading2, t.heading3, t.heading4],
            fontWeight: 'bold',
            color: '#5e81ac',
        },
        { tag: [t.heading5, t.heading6], color: '#5e81ac' },
        { tag: [t.atom, t.bool, t.special(t.variableName)], color: '#d08770' },
        { tag: [t.processingInstruction, t.inserted], color: '#8fbcbb' },
        { tag: [t.contentSeparator], color: '#ebcb8b' },
        { tag: t.invalid, color: '#434c5e', borderBottom: `1px dotted #d30102` },
    ]));

    const editorState = EditorState.create({
        doc: originalEditor.value,
        extensions: [
            lineNumbers(),
            theme,
            style,
            yaml(),
            linter(yamlSchemaLinter(traefikSchema), {}),
            lintGutter({}),
            EditorView.lineWrapping,
            keymap.of(defaultKeymap),
        ],
    });

    const editorContainer = document.createElement("div");
    originalEditor.parentNode.insertBefore(editorContainer, originalEditor);

    const editorView = new EditorView({
        state: editorState,
        parent: editorContainer,
    });

    editorView.dispatch = ((originalDispatch) => (transaction) => {
        originalDispatch(transaction);

        if (transaction.docChanged) {
            originalEditor.value = editorView.state.doc.toString();
        }
    })(editorView.dispatch);
}

function yamlSchemaLinter(schema) {
    const ajv = new AJV()
    const validate = ajv.compile(schema)

    return (view) => {
        const text = view.state.doc.toString();
        if (!text) {
            return [];
        }

        const getOffset = (line, column) => {
            return view?.state.doc.line(line + 1).from + column
        }

        const parsed = parseWithPointers(text)
        const diagnostics = parsed.diagnostics.map(diagnostic => {
            return ({
                from: getOffset(diagnostic.range.start.line, diagnostic.range.start.character),
                to: getOffset(diagnostic.range.end.line, diagnostic.range.end.character),
                severity: "error",
                message: diagnostic.message,
            })
        })

        if (diagnostics.length) {
            return diagnostics
        }

        if (!validate(parsed.data) && validate.errors) {
            const output = betterAjvErrors(schema, parsed.data, validate.errors, {format: "js"});

            return output.map(error => {
                const pointer = parseJSONPointer(error.path)
                const node = findPathOffset(parsed.ast, pointer)

                return {
                    from: node?.from || 0,
                    to: node?.to || 0,
                    severity: "error",
                    message: error.error,
                }
            })
        }

        return []
    }
}

function findPathOffset(node,  path) {
    node = findNodeAtPath(node, path)
    if (node == null) {
        return null
    }

    if (node.parent && node.parent.kind === Kind.MAPPING) {
        return {
            from: node.parent.key.startPosition,
            to: node.parent.key.endPosition,
        }
    }

    return {
        from: node.startPosition,
        to: node.endPosition,
    }
}

function findNodeAtPath(node, path) {
    pathLoop: for (const segment of path) {
        if (node == null) {
            return null
        }

        switch (node && node.kind) {
            case Kind.MAP:
                for (const item of node.mappings) {
                    if (item.key.value === segment) {
                        node = (item.value === null) ? item.key : item.value

                        continue pathLoop;
                    }
                }

                return null
            case Kind.SEQ:
                for (let i = 0; i < node.items.length; i++) {
                    if (i === Number(segment)) {
                        node = node.items[i];

                        continue pathLoop;
                    }
                }

                return null
            default:
                return null
        }
    }

    return node;
}

function parseJSONPointer(pointer) {
    if (!pointer) {
        return [];
    }

    return pointer
        .split('/')
        .slice(1)
        .map(pointer => pointer
            .split('~1')
            .join('/')
            .split('~0')
            .join('~')
        );
}

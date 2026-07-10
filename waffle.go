// Package waffle renders PDF documents from a React-authored element tree.
//
// Documents are written in ReactJS exactly as react-pdf users write them today.
// waffle can consume them two ways:
//
//   - In-process (recommended): LoadTemplate/RenderReact transpile a JSX/TSX
//     source with esbuild and execute it with the goja JavaScript engine — real
//     React runs inside the Go process, with no Node.js at render time. goja is
//     waffle's single JS engine; there is no sidecar.
//   - Static tree: RenderTree ingests a waffle-tree/v1 JSON document (the
//     serialized element tree), for setups that prefer to produce the tree in a
//     separate step. This is the same contract the in-process path emits.
//
// Both paths feed the same Go pipeline: parse the contract, build and validate
// the element tree, lay it out (flexbox + text + pagination), and paint the PDF.
// See PLAN.md for the full architecture and phase plan.
package waffle

// Version is the current waffle library version.
const Version = "0.0.0-dev"

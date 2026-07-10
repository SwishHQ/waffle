// waffle-react.js — the @waffle/react runtime: component primitives, the
// single-pass renderer (hooks dispatcher + context stack), Font/StyleSheet, and
// the serializer that emits waffle-tree/v1 (incl. {$cb} render-prop callbacks).
//
// This is THE source of truth. waffle is a pure Go library with no npm package;
// the esbuild plugin in jsruntime.go resolves the bare import `@waffle/react` to
// this embedded module, so user JSX importing from '@waffle/react' works with
// nothing installed. See assets/VENDOR.md.
import React from 'react';

// --- primitives: a component *is* its react-pdf primitive string ------------
export const Document = 'DOCUMENT';
export const Page = 'PAGE';
export const View = 'VIEW';
export const Text = 'TEXT';
export const Image = 'IMAGE';
export const ImageBackground = 'IMAGE_BACKGROUND';
export const Link = 'LINK';
export const Note = 'NOTE';
export const Canvas = 'CANVAS';

export const Svg = 'SVG';
export const G = 'G';
export const Path = 'PATH';
export const Rect = 'RECT';
export const Circle = 'CIRCLE';
export const Ellipse = 'ELLIPSE';
export const Line = 'LINE';
export const Polyline = 'POLYLINE';
export const Polygon = 'POLYGON';
export const Tspan = 'TSPAN';
export const Defs = 'DEFS';
export const ClipPath = 'CLIP_PATH';
export const LinearGradient = 'LINEAR_GRADIENT';
export const RadialGradient = 'RADIAL_GRADIENT';
export const Stop = 'STOP';

export const TextInput = 'TEXT_INPUT';
export const Checkbox = 'CHECKBOX';
export const Select = 'SELECT';
export const List = 'LIST';
export const FieldSet = 'FIELD_SET';

// --- render: resolve a React element into a host-node tree -------------------
// Rendered as a pure function of props in one synchronous pass, with a hooks
// dispatcher + context stack installed. Setters and effects are inert: a
// document is captured once, so useState returns its initial value and useEffect
// does not fire (matching how react-pdf produces a tree).
const PROVIDER = Symbol.for('react.provider');
const CONTEXT = Symbol.for('react.context');
const FORWARD_REF = Symbol.for('react.forward_ref');
const MEMO = Symbol.for('react.memo');

const ctxStack = new Map();

function pushCtx(ctx, value) {
  let s = ctxStack.get(ctx);
  if (!s) {
    s = [];
    ctxStack.set(ctx, s);
  }
  s.push(value);
}

function popCtx(ctx) {
  const s = ctxStack.get(ctx);
  if (s) s.pop();
}

function readCtx(ctx) {
  const s = ctxStack.get(ctx);
  if (s && s.length) return s[s.length - 1];
  return ctx ? ctx._currentValue : undefined;
}

let idCounter = 0;
const noop = () => {};

const dispatcher = {
  useState: (init) => [typeof init === 'function' ? init() : init, noop],
  useReducer: (reducer, initialArg, init) => [init ? init(initialArg) : initialArg, noop],
  useMemo: (fn) => fn(),
  useCallback: (fn) => fn,
  useRef: (init) => ({ current: init }),
  useContext: (ctx) => readCtx(ctx),
  useEffect: noop,
  useLayoutEffect: noop,
  useInsertionEffect: noop,
  useImperativeHandle: noop,
  useDebugValue: noop,
  useId: () => ':f' + idCounter++ + ':',
  useSyncExternalStore: (_subscribe, getSnapshot) => getSnapshot(),
  useTransition: () => [false, (fn) => fn()],
  useDeferredValue: (v) => v,
};

// callbacks registry: function render-props are kept here (in the VM) and
// referenced from the tree as {$cb:id} so the Go engine can evaluate them per
// page. Populated during serialization (see cleanProps).
let callbacks = {};
let cbCounter = 0;

function installDispatcher(fn) {
  const internals = React.__SECRET_INTERNALS_DO_NOT_USE_OR_YOU_WILL_BE_FIRED;
  const cur = internals && internals.ReactCurrentDispatcher;
  const prev = cur ? cur.current : null;
  if (cur) cur.current = dispatcher;
  try {
    return fn();
  } finally {
    if (cur) cur.current = prev;
  }
}

// render is the top-level document pass: it resets per-render state (context
// stack, hook ids, callback registry) then resolves the tree.
export function render(element) {
  ctxStack.clear();
  idCounter = 0;
  callbacks = {};
  cbCounter = 0;
  return installDispatcher(() => renderElement(element));
}

// resolveResult renders the output of a function render-prop. It keeps the
// callback registry intact (only context is reset — a callback runs detached
// from its tree position) so evaluating one callback cannot clobber the others.
function resolveResult(element) {
  ctxStack.clear();
  return installDispatcher(() => renderElement(element));
}

export function renderElement(el) {
  if (el == null || typeof el === 'boolean') return null;
  if (typeof el === 'string' || typeof el === 'number') {
    return { type: 'TEXT_INSTANCE', value: String(el) };
  }
  if (Array.isArray(el)) {
    return el.flatMap((c) => {
      const n = renderElement(c);
      return Array.isArray(n) ? n : n ? [n] : [];
    });
  }

  const { type, props = {} } = el;

  if (type === React.Fragment) {
    return renderChildren(props.children);
  }
  if (typeof type === 'function') {
    if (type.prototype && type.prototype.isReactComponent) {
      const inst = new type(props);
      return renderElement(inst.render());
    }
    return renderElement(type(props));
  }

  if (type && typeof type === 'object') {
    switch (type.$$typeof) {
      case PROVIDER: {
        const ctx = type._context;
        pushCtx(ctx, props.value);
        try {
          return renderChildren(props.children);
        } finally {
          popCtx(ctx);
        }
      }
      case CONTEXT: {
        const value = readCtx(type._context || type);
        const child = typeof props.children === 'function' ? props.children(value) : props.children;
        return renderElement(child);
      }
      case FORWARD_REF:
        return renderElement(type.render(props, props.ref || null));
      case MEMO:
        return renderElement({ type: type.type, props, key: el.key });
    }
  }

  const { children, ...rest } = props;
  return { type: String(type), props: rest, children: renderChildren(children) };
}

export function renderChildren(children) {
  const out = [];
  const visit = (c) => {
    if (c == null || c === false || c === true) return;
    if (Array.isArray(c)) {
      c.forEach(visit);
      return;
    }
    if (typeof c === 'string' || typeof c === 'number') {
      out.push({ type: 'TEXT_INSTANCE', value: String(c) });
      return;
    }
    const node = renderElement(c);
    if (Array.isArray(node)) node.forEach((n) => n && out.push(n));
    else if (node) out.push(node);
  };
  visit(children);
  return out;
}

// --- Font / StyleSheet shims (react-pdf API surface) -------------------------
let fonts = [];
let emojiSource = null;

export const Font = {
  register(cfg) {
    const faces = cfg.fonts
      ? cfg.fonts
      : [{ src: cfg.src, fontWeight: cfg.fontWeight, fontStyle: cfg.fontStyle }];
    fonts.push({
      family: cfg.family,
      fonts: faces.map((f) => ({
        src: f.src,
        fontWeight: f.fontWeight,
        fontStyle: f.fontStyle,
      })),
    });
  },
  registerEmojiSource(src) {
    emojiSource = src;
  },
  registerHyphenationCallback() {
    // Requires per-line VM evaluation; ignored while producing a static tree.
  },
  clear() {
    fonts = [];
    emojiSource = null;
  },
};

export const StyleSheet = {
  create: (styles) => styles,
  flatten: (style) =>
    Array.isArray(style) ? Object.assign({}, ...style.filter(Boolean)) : style || {},
};

// --- serialize: React element → waffle-tree/v1 -------------------------------
// cleanProps drops function-valued props, EXCEPT recognized callback props
// (currently `render`): those are registered in the VM and replaced with a
// {$cb:id} ref the Go engine evaluates per page. onRender and other non-content
// function props are still dropped.
const CALLBACK_PROPS = { render: true, paint: true };

function cleanProps(props) {
  const out = {};
  for (const [k, v] of Object.entries(props || {})) {
    if (typeof v === 'function') {
      if (CALLBACK_PROPS[k]) {
        const id = 'cb_' + cbCounter++;
        callbacks[id] = v;
        out[k] = { $cb: id };
      }
      continue;
    }
    out[k] = v;
  }
  return out;
}

function toNode(n) {
  if (n.type === 'TEXT_INSTANCE') {
    return { type: 'TEXT_INSTANCE', value: n.value };
  }
  return {
    type: n.type,
    props: cleanProps(n.props),
    children: (n.children || []).map(toNode),
  };
}

export function serialize(element) {
  const rendered = render(element);
  const doc = Array.isArray(rendered)
    ? rendered.find((n) => n && n.type === 'DOCUMENT')
    : rendered;
  if (!doc || doc.type !== 'DOCUMENT') {
    throw new Error('@waffle/react: the root element must be a <Document>');
  }

  const document = {
    props: cleanProps(doc.props),
    children: (doc.children || []).map(toNode),
  };
  if (fonts.length) document.fonts = fonts.map((f) => ({ ...f }));
  if (emojiSource) document.emojiSource = emojiSource;

  const tree = { version: 'waffle-tree/v1', document };
  const ids = Object.keys(callbacks);
  if (ids.length) tree.callbacks = ids;
  return tree;
}

export function serializeString(element) {
  return JSON.stringify(serialize(element));
}

// evalCallbackString evaluates a registered render-prop callback by id with the
// given page context (JSON) and returns a JSON array of waffle-tree nodes. The
// Go engine calls this per page during pagination.
export function evalCallbackString(id, ctxJSON) {
  const fn = callbacks[id];
  if (!fn) throw new Error('@waffle/react: unknown callback ' + id);
  const ctx = ctxJSON ? JSON.parse(ctxJSON) : {};
  const resolved = resolveResult(fn(ctx));
  const list = Array.isArray(resolved) ? resolved : resolved ? [resolved] : [];
  return JSON.stringify(list.map(toNode));
}

// makePainter returns a fluent recorder matching react-pdf's Canvas painter API.
// Each drawing call pushes {op, args} in the exact shape the Go canvas replayer
// reads. Methods the Go side doesn't implement (gradients, opacity) still record
// harmlessly — the replayer ignores unknown ops.
function makePainter() {
  const ops = [];
  const painter = { _ops: ops };
  const record = (name) => (...args) => {
    ops.push({ op: name, args });
    return painter;
  };
  const methods = [
    'moveTo', 'lineTo', 'bezierCurveTo', 'quadraticCurveTo', 'rect', 'circle',
    'ellipse', 'polygon', 'path', 'closePath', 'fill', 'stroke', 'fillAndStroke',
    'lineWidth', 'lineCap', 'lineJoin', 'fillColor', 'strokeColor', 'save',
    'restore', 'translate', 'scale', 'rotate', 'fillOpacity', 'strokeOpacity',
    'opacity', 'dash', 'transform', 'linearGradient', 'radialGradient',
  ];
  for (const name of methods) painter[name] = record(name);
  return painter;
}

// evalPaintString runs a Canvas paint callback against a fresh painter of the
// given size and returns the recorded ops as JSON. react-pdf signature:
// paint(painter, availableWidth, availableHeight).
export function evalPaintString(id, w, h) {
  const fn = callbacks[id];
  if (!fn) throw new Error('@waffle/react: unknown paint callback ' + id);
  const painter = makePainter();
  installDispatcher(() => fn(painter, w, h));
  return JSON.stringify(painter._ops);
}

package jsruntime

import (
	"strings"
	"testing"
)

// renderTree compiles a JSX document and returns its feast-tree JSON, failing
// the test on any error. Used to prove React hooks/context run under goja.
func renderTree(t *testing.T, doc string) string {
	t.Helper()
	prog, err := Compile([]byte(doc), Options{})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	out, err := prog.Render(nil)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	return string(out)
}

func TestGojaUseState(t *testing.T) {
	const doc = `
import { Document, Page, Text } from '@feast/react';
import { useState } from 'react';
export default function App() {
  const [n] = useState(41);
  const [label] = useState(() => 'ready');
  return <Document><Page><Text>{label + ' ' + (n + 1)}</Text></Page></Document>;
}
`
	if s := renderTree(t, doc); !strings.Contains(s, "ready 42") {
		t.Errorf("useState under goja failed; tree=%s", s)
	}
}

func TestGojaUseContext(t *testing.T) {
	const doc = `
import { Document, Page, View, Text } from '@feast/react';
import React from 'react';
const Theme = React.createContext('light');
function Label() {
  const theme = React.useContext(Theme);
  return <Text>{'theme=' + theme}</Text>;
}
export default function App() {
  return (
    <Document>
      <Page>
        <Label />
        <Theme.Provider value="dark">
          <View><Label /></View>
        </Theme.Provider>
      </Page>
    </Document>
  );
}
`
	s := renderTree(t, doc)
	if !strings.Contains(s, "theme=light") {
		t.Errorf("default context value missing; tree=%s", s)
	}
	if !strings.Contains(s, "theme=dark") {
		t.Errorf("provider context value missing; tree=%s", s)
	}
}

func TestGojaMemoAndForwardRef(t *testing.T) {
	const doc = `
import { Document, Page, Text } from '@feast/react';
import React from 'react';
const Memo = React.memo(function Memo(){ return <Text>memo-ok</Text>; });
const Fwd = React.forwardRef(function Fwd(props, ref){ return <Text>{'fref-' + props.n}</Text>; });
export default function App() {
  return <Document><Page><Memo/><Fwd n="1"/></Page></Document>;
}
`
	s := renderTree(t, doc)
	for _, want := range []string{"memo-ok", "fref-1"} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q; tree=%s", want, s)
		}
	}
	if strings.Contains(s, "[object Object]") {
		t.Errorf("exotic element type leaked as [object Object]; tree=%s", s)
	}
}

func TestGojaUseMemoReducerRef(t *testing.T) {
	const doc = `
import { Document, Page, Text } from '@feast/react';
import { useMemo, useReducer, useRef } from 'react';
export default function App() {
  const v = useMemo(() => 6 * 7, []);
  const [count] = useReducer((x) => x + 1, 100);
  const r = useRef('R');
  return <Document><Page><Text>{v + ' ' + count + ' ' + r.current}</Text></Page></Document>;
}
`
	if s := renderTree(t, doc); !strings.Contains(s, "42 100 R") {
		t.Errorf("useMemo/useReducer/useRef under goja failed; tree=%s", s)
	}
}

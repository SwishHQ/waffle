// appointment.jsx — a full offer/appointment letter for a fictional company
// (Northwind Foods), exercising feast end-to-end: A4 geometry with 2" letterhead
// margins, a full-page letterhead background, embedded fonts, numbered clauses,
// inline bold/italic runs, a signature image, an offer-acceptance block, and a
// prop-driven Annexure A compensation table.
//
// Render from this directory so the relative asset paths resolve:
//   feast appointment.jsx appointment.pdf
//
// Placeholder tokens (<Candidate Name>, <Salary>, ...) are overridable via props.
import { Document, Page, View, Text, Image, Font } from '@feast/react';

// The Go fonts (Bigelow & Holmes, BSD-licensed — see assets/fonts/NOTICE.md) in
// all four faces feast needs to resolve fontWeight/fontStyle to the right TTF.
Font.register({
  family: 'Go',
  fonts: [
    { src: 'assets/fonts/Go-Regular.ttf', fontWeight: 'normal', fontStyle: 'normal' },
    { src: 'assets/fonts/Go-Bold.ttf', fontWeight: 'bold', fontStyle: 'normal' },
    { src: 'assets/fonts/Go-Italic.ttf', fontWeight: 'normal', fontStyle: 'italic' },
    { src: 'assets/fonts/Go-BoldItalic.ttf', fontWeight: 'bold', fontStyle: 'italic' },
  ],
});

// A4 in points, and the DOCX page margins (top/bottom 2", sides 1").
const A4 = { w: 595.28, h: 841.89 };
const MARGIN = { top: 144, bottom: 144, x: 72 };

// --- run helpers: a paragraph is an array of strings (normal) or tagged spans.
const b = (t) => ({ b: t }); // bold
const iu = (t) => ({ iu: t }); // italic + underlined
const bs = { fontWeight: 'bold' };
const ius = { fontStyle: 'italic', textDecoration: 'underline' };

function Runs({ parts }) {
  // Mixed string/element children become inline styled runs sharing lines.
  return parts.map((p, i) => {
    if (typeof p === 'string') return p;
    if (p.b !== undefined) return <Text key={i} style={bs}>{p.b}</Text>;
    if (p.iu !== undefined) return <Text key={i} style={ius}>{p.iu}</Text>;
    return String(p);
  });
}

// Justified body paragraph.
function P({ parts, style }) {
  return (
    <Text style={{ marginBottom: 8, textAlign: 'justify', ...style }}>
      <Runs parts={parts} />
    </Text>
  );
}

// Bold, auto-numbered section heading (1., 2., ...).
function Heading({ n, title }) {
  return (
    <View style={{ flexDirection: 'row', marginTop: 12, marginBottom: 6 }} wrap={false}>
      <Text style={{ width: 22, fontWeight: 'bold' }}>{n}.</Text>
      <Text style={{ flex: 1, fontWeight: 'bold' }}>{title}</Text>
    </View>
  );
}

// A numbered/lettered clause with a hanging indent: label column + justified body.
function Clause({ label, parts, gap = 26 }) {
  return (
    <View style={{ flexDirection: 'row', marginBottom: 7, paddingLeft: 24 }}>
      <Text style={{ width: gap }}>{label}</Text>
      <Text style={{ flex: 1, textAlign: 'justify' }}>
        <Runs parts={parts} />
      </Text>
    </View>
  );
}

export default function AppointmentLetter(props = {}) {
  const p = {
    date: '<DD-MM-YYYY>',
    candidate: '<Candidate Name>',
    father: '<Father’s Name>',
    addr1: '<Address Line 1>',
    addr2: '<Address Line 2>',
    pincode: '<Pincode>',
    name: '<Name>',
    designation: '<Designation>',
    joining: '<Date of Joining>',
    location: '<Location>',
    salary: '<Salary>',
    location2: '<Location 2>',
    employee: '<Name of Employee>',
    acceptDate: '<Date>',
    ...props,
  };
  const COMPANY = 'Northwind Foods Private Limited (the “Company”)';

  // Full-bleed letterhead: absolute insets resolve against the content box, so
  // negative offsets push it out to the page corner; `fixed` repeats it per page.
  const Letterhead = () => (
    <Image
      fixed
      src="assets/letterhead.png"
      style={{ position: 'absolute', top: -MARGIN.top, left: -MARGIN.x, width: A4.w, height: A4.h, zIndex: -1 }}
    />
  );

  const pageStyle = {
    fontFamily: 'Go',
    fontSize: 12,
    lineHeight: 1.15,
    color: '#000000',
    paddingTop: MARGIN.top,
    paddingBottom: MARGIN.bottom,
    paddingLeft: MARGIN.x,
    paddingRight: MARGIN.x,
  };

  return (
    <Document title="Appointment Letter" author="Northwind Foods Private Limited" subject="Appointment Letter">
      {/* ---- Letter body ---- */}
      <Page size="A4" style={pageStyle}>
        <Letterhead />

        <Text style={{ textAlign: 'center', fontWeight: 'bold', marginBottom: 10 }}>APPOINTMENT LETTER</Text>
        <Text style={{ textAlign: 'right', marginBottom: 10 }}>{p.date}</Text>

        <Text style={{ fontWeight: 'bold' }}>To,</Text>
        <Text>{p.candidate}</Text>
        <Text>{p.father}</Text>
        <Text>{p.addr1}</Text>
        <Text>{p.addr2}</Text>
        <Text style={{ marginBottom: 10 }}>{p.pincode}</Text>

        <P parts={['Dear ', b(p.name), ',']} />
        <P parts={['We are pleased to offer you the position of ', b(p.designation), ' at ', b(COMPANY), ' on the terms detailed below.']} />

        <Heading n={1} title="Date of Joining, Training and Background Check" />
        <Clause label="1.1" parts={['Your employment with the Company will commence on ', b(p.joining), ' (the “', b('Date of Joining'), '”) unless terminated (“', b('Term'), '”).']} />
        <Clause label="1.2" parts={['For the first fifteen (15) days of your employment (“', b('Training Period'), '”), you will be in your training period. During this period, the Company will assess your performance, conduct and suitability for the role. At any time during the training, the Company may reduce or extend the Training Period at its sole discretion. Unless communicated otherwise in writing, confirmation will be deemed automatic upon the completion of the Training Period.']} />
        <Clause label="1.3" parts={['This appointment is based on the information shared in your application and during the interview. By accepting this offer, you confirm that all information provided by you is accurate and complete to the best of your knowledge. Your employment is subject to the submission of required documents and successful completion of any pre- or post-employment background checks that the Company may carry out. Please note that if any pre- and/or post-employment background checks result in an adverse or unsatisfactory report, this appointment shall stand revoked automatically, and if the employment has already commenced, such employment may be terminated without any notice or compensation in lieu of notice. By signing this Appointment Letter, you hereby agree to authorize verification and background check and agree to sign any and all documents necessary to enable the Company to conduct this verification and background check.']} />

        <Heading n={2} title="Location, Reporting and Duties" />
        <Clause label="2.1" parts={['Your posting location shall be ', b(p.location), '. However, the Company reserves the right to transfer you to any other location of the Company within India.']} />
        <Clause label="2.2" parts={['Your duties and responsibilities in the Company will be limited to the tasks assigned to you in connection with the Company’s operations. You shall devote full working time to the Company.']} />

        <Heading n={3} title="Working Hours and Leave Entitlement" />
        <Clause label="3.1" parts={['Your working hours shall be in accordance with a 48-hour work week, with shift timings determined based on business needs. Your schedule may vary depending on operational requirements.']} />
        <Clause label="3.2" parts={['Your remuneration is inclusive of compensation for all such working hours, subject to applicable labour laws. Any hours worked outside of regular working hours are subject to the prior written approval of your Shift/Pod/Cluster Manager.']} />
        <Clause label="3.3" parts={['You shall be entitled to eighteen (18) days of paid leave and twelve (12) days of sick cum casual leave per calendar year, subject to the Company leave policy and approval processes.']} />

        <Heading n={4} title="Compensation and Statutory Benefits" />
        <Clause label="4.1" parts={['Your fixed compensation shall be INR ', b(p.salary), ' per annum, payable in monthly instalments, subject to statutory deductions. The detailed salary structure is set out in ', b('Annexure A'), '. Payments are subject to withholding of applicable taxes, statutory deductions, and adjustment of any outstanding advances or loans, as per applicable laws. The Company reserves the right to restructure salary components without reducing gross compensation.']} />
        <Clause label="4.2" parts={['The Company shall make statutory contributions towards Provident Fund, Employees State Insurance (ESI), and other benefits as applicable under law. You agree to provide accurate details such as Aadhaar linked UAN, bank account, etc. The Company shall deduct and remit statutory contributions as per the applicable laws. You acknowledge that PF contributions are subject to successful Aadhaar seeding, and that any delay due to incorrect or incomplete information provided by you shall be your sole responsibility, and the Company shall not be liable for any penalties, damages, or interest in relation to the same.']} />
        <Clause label="4.3" parts={['Each Party shall be responsible for its respective tax liabilities arising under this Appointment Letter. Any increase in your tax or social security liability due to a change in applicable law shall not obligate the Company to restructure your remuneration or provide any compensation.']} />
        <Clause label="4.4" parts={['The Company provides maternity benefits to eligible employees in accordance with the Code on Social Security 2020.']} />

        <Heading n={5} title="Company Policies" />
        <P style={{ paddingLeft: 24 }} parts={['You shall comply with all Company rules, policies, and standard operating procedures, including those relating to attendance, discipline, health, safety, hygiene, code of conduct, prevention of sexual harassment, confidentiality and data protection, as may be revised from time to time. In case of any conflict between the provisions of this Appointment Letter and the policies, rules and regulations of the Company, the provisions of this Appointment Letter, shall prevail.']} />

        <Heading n={6} title="Termination of Employment" />
        <Clause label="(a)" parts={['On and from the Joining Date until the expiry of the Training Period, either you or the Company may terminate this employment by providing 7 (seven) days’ prior written notice to the other party, or by payment of salary in lieu of such notice period.']} />
        <Clause label="(b)" parts={['Following your Training Period, either you or the Company may terminate your employment at any time by providing 15 days notice period or payment of salary in lieu of such notice.']} />
        <Clause label="(c)" parts={['It is clarified that if you intend to terminate your employment, a written resignation will have to be accepted by the Company to be considered effective. Any acceptance of resignation will be on an irrevocable basis. The Company may decline to accept your resignation where an internal inquiry is pending or during deputation or secondment.']} />
        <Clause label="(d)" parts={['The Company reserves the right to terminate your employment without notice or pay in lieu of notice period in cases of gross misconduct, fraud, health or safety standards violations, intoxication during working hours, habitual or unauthorised absence, breach of Company policy, dishonesty, theft, gross negligence in the performance of your duties, willful disobedience of lawful orders, use of intoxicants during working hours, or any other reason amounting to “cause,” as determined by the Company’s sole discretion and applicable laws.']} />
        <Clause label="(e)" parts={['You agree that in case of retrenchment, the principle of “Last in First Out” will not be applicable.']} />
        <Clause label="(f)" parts={['Upon cessation of employment, you shall immediately return all Company property, including uniforms, ID cards, tools, equipment, documents, and any other assets in your possession.']} />
        <Clause label="(g)" parts={['No leave shall ordinarily be permitted during the notice period, unless approved by the Company.']} />

        <Heading n={7} title="Confidentiality" />
        <P style={{ paddingLeft: 24 }} parts={['You shall maintain strict confidentiality during and after employment with respect to all Company information which is confidential, including but not limited to processes, supplier details, pricing, customer data, and operational methods, and shall not use, copy, reproduce, or remove any Confidential Information for your own or any third party’s benefit without authorization.']} />

        <Heading n={8} title="Data Protection" />
        <Clause label="8.1" parts={['You shall, at all times during your employment, comply with the Company’s data protection policies, information security standards, and process personal data of employees, customers, clients, vendors, supplier, agent, or any other individual whose personal data you may access in the course of your employment. You shall process personal data only for authorised purposes and in accordance with applicable law, including the Digital Personal Data Protection Act, 2023 and the rules framed thereunder (“', b('Data Protection Laws'), '”).']} />
        <Clause label="8.2" parts={[b('Consent for Processing of Employee Personal Data. '), 'You expressly consent and authorise the Company, in its capacity as a Data Fiduciary, to collect and process your personal data for the purposes described below, in accordance with the Data Protection Laws:']} />
        <Clause label="(i)" gap={30} parts={[iu('Collection and Processing'), ': To collect, use, access, store, retain, disclose, and otherwise process your personal data (whether provided by you, collected automatically, or generated during your employment) for purposes including: (i) pre-employment verification and background checks; (ii) onboarding and creation of employment records; (iii) administration of payroll, compensation, benefits, performance management, and leave; (iv) compliance with applicable law, internal policies, reporting obligations, and audits; (v) disciplinary proceedings, investigations, and ensuring workplace safety; and (vi) processing activities required at the time of your separation.']} />
        <Clause label="(ii)" gap={30} parts={[iu('Sources of Collection'), ': Your personal data may be collected (i) directly from you; (ii) from third-party verification agencies; (iii) from internal systems and platforms used during employment; and (iv) from any records generated in the course of your employment or separation.']} />
        <Clause label="8.3" parts={[b('Sharing, and Disclosure. '), 'You consent to the Company sharing or transferring your personal data for the purposes stated above with: (a) internal departments, group companies, and affiliates of the Company, whether located in India or outside India; (b) governmental or regulatory authorities when disclosure is mandated under applicable law or by order, direction, or investigation; (c) third-party service providers (including HR, payroll processors, auditors, IT service providers, background verification agencies, or advisors) who are bound by contractual and confidentiality obligations; (d) external advisors, consultants, or parties involved in any corporate transaction (including merger, acquisition, restructuring, or sale of business or assets); and (e) in internal Company communications or directories, where reasonably necessary for business operations.']} />

        <Heading n={9} title="Intellectual Property" />
        <P style={{ paddingLeft: 24 }} parts={['Any work, process, method, know-how, secret process or material developed by you whether or not capable of protection under the intellectual property laws, that is made, conceived, developed, whether alone or jointly with others during the course of your employment and in connection with your employment shall be deemed to be a “work made for hire” and shall be the exclusive property of the Company. If any work does not qualify as a work made for hire, you irrevocably assign all rights, title, and interest, including intellectual property rights, to the Company worldwide and in perpetuity, without additional compensation. You agree to take all actions to give effect to this assignment, including after employment ends, and waive all moral rights and rights under the Copyright Act, 1957.']} />

        <Heading n={10} title="Other General Terms and Conditions" />
        <Clause label="10.1" gap={30} parts={['You agree that during your Term and after employment you shall not, directly or indirectly, make or assist in making any public or private statement, in writing, orally or electronically, that disparages, or harms the reputation of the Company, its policies, its employees or products.']} />
        <Clause label="10.2" gap={30} parts={['You shall indemnify and hold harmless the Company, its affiliates and their respective directors, employees, officers, from all claims, losses and expenses (including reasonable legal fees) arising from your fraud, misrepresentation, breach of applicable law, Appointment Letter or Company Policy.']} />
        <Clause label="10.3" gap={30} parts={['This Appointment Letter shall be governed by the laws of India. Courts at ', b(p.location2), ' shall have exclusive jurisdiction.']} />

        <P style={{ marginTop: 8 }} parts={['Please return a signed copy of this Appointment Letter within 3 working days to confirm your acceptance. If the signed copy is not received within this period, the offer will automatically lapse without further notice. Further, if after accepting the offer you fail to join on the Date of Joining or any mutually written agreed date, the offer will stand automatically withdrawn.']} />

        {/* Signature block */}
        <View style={{ marginTop: 18 }} wrap={false}>
          <Text>For and on behalf of</Text>
          <Text style={{ fontWeight: 'bold' }}>NORTHWIND FOODS PRIVATE LIMITED</Text>
          <Image src="assets/signature.png" style={{ width: 130, height: 48, marginVertical: 4 }} />
          <Text>______________________</Text>
          <Text style={{ fontWeight: 'bold' }}>Priya Sharma</Text>
          <Text>Director - People Operations</Text>
          <Text>Northwind</Text>
        </View>

        {/* two blank lines above the acceptance block */}
        <Text> </Text>
        <Text> </Text>

        {/* Offer acceptance */}
        <View style={{ marginTop: 28 }} wrap={false}>
          <Text style={{ fontWeight: 'bold', marginBottom: 10 }}>Offer Acceptance</Text>
          <Text style={{ marginBottom: 24 }}>
            <Runs parts={['I, ', b(p.employee), ', hereby accept this offer as ', b(p.designation), ' and confirm my Date of Joining start date as ', b(p.acceptDate)]} />
          </Text>
          <Text style={{ marginBottom: 4 }}>Signature: ____________________</Text>
          <Text style={{ fontWeight: 'bold' }}>{p.employee}</Text>
        </View>
      </Page>

      {/* ---- Annexure A ---- */}
      <Page size="A4" style={pageStyle}>
        <Letterhead />
        <Text style={{ textAlign: 'center', fontWeight: 'bold' }}>Annexure A:</Text>
        <Text style={{ textAlign: 'center', fontWeight: 'bold', marginBottom: 16 }}>Compensation Details</Text>

        <CompensationTable comp={p.comp || {}} />

        {/* Statutory notes — forced onto the page after the table. */}
        <View break>
          <Text style={{ marginBottom: 6 }}>Gratuity would be paid as per Code on Social Security, 2020.</Text>
          <Text style={{ marginBottom: 6 }}>For the Provident Fund, the contribution will be payable as per the provisions of the Employees’ Provident Funds &amp; Miscellaneous Provisions Act, 1952.</Text>
          <Text>Take home salary will be net after PF &amp; Income Tax and any other statutory deductions depending on your savings under various schemes.</Text>
        </View>
      </Page>
    </Document>
  );
}

// The Annexure A compensation table: an Earnings block and a Deductions block,
// each with Particulars/Monthly/Annual columns, rendered as a thin-ruled grid
// with the placeholder tokens the template carries. Cells are transparent so the
// letterhead watermark shows through, matching the source document.
const TABLE_BORDER = '0.75pt solid #9c9578';
// Column flex weights: the Monthly column is narrower than Particulars/Annual.
const COL = { p: 1.6, m: 1, a: 1.6 };

// Cell wraps its content with a right/bottom rule and vertically-centered text
// (justifyContent centers on the column main-axis). flexBasis:0 makes the column
// width depend only on the flex ratio, not the content — so every row's columns
// line up into a true grid (feast's `flex:N` shorthand leaves flex-basis auto).
function Cell({ flex, bold, size, children }) {
  return (
    <View
      style={{
        flexGrow: flex,
        flexShrink: 1,
        flexBasis: 0,
        borderRight: TABLE_BORDER,
        borderBottom: TABLE_BORDER,
        paddingVertical: 6,
        paddingHorizontal: 6,
        justifyContent: 'center',
      }}
    >
      <Text style={{ textAlign: 'center', fontWeight: bold ? 'bold' : 'normal', fontSize: size }}>{children}</Text>
    </View>
  );
}

// A full-width section band (Earnings / Deductions).
function Band({ children }) {
  return (
    <View style={{ flexDirection: 'row' }}>
      <Cell flex={1} bold size={13}>{children}</Cell>
    </View>
  );
}

// The Particulars / Monthly / Annual column header.
function SubHead() {
  return (
    <View style={{ flexDirection: 'row' }}>
      <Cell flex={COL.p} bold>Particulars</Cell>
      <Cell flex={COL.m} bold>Monthly</Cell>
      <Cell flex={COL.a} bold>Annual</Cell>
    </View>
  );
}

// A data row: a (optionally bold) label and its monthly/annual value tokens.
function TRow({ label, m, a, bold }) {
  return (
    <View style={{ flexDirection: 'row' }}>
      <Cell flex={COL.p} bold={bold}>{label}</Cell>
      <Cell flex={COL.m} bold size={10.5}>{m}</Cell>
      <Cell flex={COL.a} bold size={10.5}>{a}</Cell>
    </View>
  );
}

// Each row: [key, label, bold?]. The key looks up the monthly/annual value from
// the `comp` prop; when a value is absent the placeholder token is shown, so the
// bare template renders with <Monthly CTC>-style placeholders.
const EARNINGS_ROWS = [
  ['ctc', 'CTC', true],
  ['basic', 'Basic'],
  ['hra', 'HRA'],
  ['special', 'Special'],
  ['gross', 'Gross earnings', true],
];
const DEDUCTION_ROWS = [
  ['pfEr', 'Employer PF'],
  ['pfEp', 'Employee PF'],
  ['pt', 'PT'],
  ['esic', 'ESIC'],
  ['totalDed', 'Total deduction'],
  ['net', 'Net Pay after deductions', true],
];
// Placeholder tokens shown when the corresponding prop value is not supplied.
const COMP_TOKENS = {
  ctc: ['<Monthly CTC>', '<Annual CTC>'],
  basic: ['<Monthly Basic>', '<Annual Basic>'],
  hra: ['<Monthly HRA>', '<Annual HRA>'],
  special: ['<Monthly Special>', '<Annual Special>'],
  gross: ['<Monthly Gross>', '<Annual Gross>'],
  pfEr: ['<Monthly PF ER>', '<Annual PF ER>'],
  pfEp: ['<Monthly PF EP>', '<Annual PF EP>'],
  pt: ['<Monthly PT>', '<Annual PT>'],
  esic: ['<Monthly ESIC>', '<Annual ESIC>'],
  totalDed: ['<Monthly Total D>', '<Annual D>'],
  net: ['<Monthly Net>', '<Annual Net>'],
};

// CompensationTable renders the Annexure A grid. Values come from the `comp`
// prop (e.g. comp.ctc = { m: '30,000', a: '3,60,000' }); missing values fall
// back to the placeholder token.
function CompensationTable({ comp = {} }) {
  const monthly = (k) => (comp[k] && comp[k].m != null ? String(comp[k].m) : COMP_TOKENS[k][0]);
  const annual = (k) => (comp[k] && comp[k].a != null ? String(comp[k].a) : COMP_TOKENS[k][1]);
  const rows = (list) =>
    list.map(([k, label, bold]) => <TRow key={k} label={label} m={monthly(k)} a={annual(k)} bold={bold} />);

  return (
    <View style={{ borderTop: TABLE_BORDER, borderLeft: TABLE_BORDER }}>
      <Band>Earnings</Band>
      <SubHead />
      {rows(EARNINGS_ROWS)}
      <Band>Deductions</Band>
      <SubHead />
      {rows(DEDUCTION_ROWS)}
    </View>
  );
}

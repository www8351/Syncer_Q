## Vertex Command - אפיון מערכת קיימת

  ### סקירה כללית
  פלטפורמת SaaS מקצועית בעברית (RTL) לניהול חשבונות מסחר נוסטרו (Prop Trading) אצל חברות מימון. מאפשרת מעקב אחרי מספר חשבונות במקביל, ניתוח ביצועים, התראות, ושליטה מלאה בתהליך.

  ### טכנולוגיה
  - **Frontend:** React + Vite, Tailwind CSS, shadcn/ui, Framer Motion, Recharts
  - **Backend:** Express.js, PostgreSQL, Drizzle ORM
  - **Auth:** Sessions בדאטהבייס, הצפנת סיסמאות (bcrypt)
  - **Email:** Gmail API (אימות + איפוס סיסמה)
  - **AI:** OpenAI (צ'אטבוט עזרה)
  - **תשלומים:** Stripe (checkout, portal, webhooks)

  ### מודולים מוכנים

  **1. מערכת הרשמה/התחברות**
  - הרשמה עם אימות מייל
  - התחברות עם session
  - איפוס סיסמה דרך מייל (תוקף 60 דקות)
  - הרשאות: משתמש רגיל / אדמין

  **2. דשבורד ראשי**
  - KPIs: סך באלאנס, רווח/הפסד, משיכות, מספר חשבונות
  - גרפים: היסטוריית באלאנס, התפלגות רווחים
  - טבלת חשבונות עם סינון לפי סטטוס/שלב/חברה
  - התראות מערכת
  - הגדרות (ניהול חברות, כללים, ייצוא נתונים)

  **3. ניהול חשבונות**
  - הוספה/עריכה/מחיקה של חשבונות מסחר
  - שדות: שם, חברה, שלב (evaluation/funded/payout), גודל, באלאנס, יעד, drawdown, consistency rule
  - סטטוס מחושב אוטומטית: healthy, buffer_building, near_target, ready_to_withdraw, consistency_risk, drawdown_risk, violated
  - היסטוריית באלאנס יומית (snapshots)
  - audit log למעקב שינויים

  **4. מנוע חוקים (Rule Engine)**
  - הגדרת חוקים לכל חברת נוסטרו (Firm)
  - תמיכה ב-Tiers (סוגי חשבונות שונים לאותה חברה)
  - חישוב Drawdown - Static ו-Trailing
  - חישוב Consistency Rule (אחוז מקסימלי מיום בודד)
  - סטטוס אוטומטי לכל חשבון בהתאם לחוקים

  **5. עדיפויות מסחר ("מה לסחור היום")**
  - ציון 0-100 לכל חשבון
  - המלצה: trade / light_trading / avoid / do_not_trade / ready_to_withdraw
  - מבוסס על: מרחק מיעד, סיכון drawdown, consistency

  **6. ניהול משיכות**
  - מעקב מלא על תהליך משיכה (בקשה → אישור → בוצע)
  - היסטוריית משיכות לכל חשבון

  **7. התראות**
  - התראות אוטומטיות (drawdown risk, consistency warning וכו')
  - סימון כנקרא, מחיקה

  **8. אינטגרציות**
  - **Tradovate** - חיבור API אמיתי עם Live/Demo, גילוי חשבונות אוטומטי
  - **TopstepX** - ממשק מוכן, חיבור API בקרוב
  - **Rithmic** - ממשק מוכן, חיבור API בקרוב
  - **CSV Import** - ייבוא נתונים מקובץ

  **9. חיוב ומנויים (Stripe)**
  - 4 תוכניות: Free / Pro ($29) / Trader ($79) / Desk ($199)
  - תוכנית שנתית עם 20% הנחה
  - Stripe Checkout לתשלום
  - Customer Portal לניהול מנוי
  - Feature gating לפי תוכנית (אינטגרציות, סנכרון, ייצוא, צוות)
  - Webhook handler לעדכון סטטוס מנוי

  **10. מודול אדמין**
  - רשימת משתמשים
  - מדדי מערכת: MRR, מנויים פעילים, סך משתמשים
  - סטטוס בריאות אינטגרציות

  **11. עזרה ו-FAQ**
  - שאלות נפוצות עם חיפוש
  - צ'אטבוט AI (OpenAI) שמבין את חוקי המערכת ונוסטרו

  **12. ייצוא נתונים**
  - CSV חשבונות
  - CSV משיכות
  - PDF מפרט מערכת

  ### דאטהבייס - 21 טבלאות
  - **Core:** users, firms, firm_tiers, accounts, withdrawals, balance_history, alerts, monthly_reports, audit_log, settings
  - **Integrations:** integration_providers, integration_connections, integration_accounts, imported_trades, sync_jobs, sync_logs
  - **Billing:** plans, subscriptions, invoices, payment_methods, billing_events

  ### מה עדיין חסר/מתוכנן
  1. **תמיכה רב-שפתית** - כרגע עברית בלבד, מתוכנן: אנגלית, ערבית, ספרדית
  2. **כפתור חיבורים בניווט מובייל** - חסר בתפריט התחתון
  3. **TopstepX / Rithmic** - API אמיתי עדיין לא מחובר
  4. **הגדרת Production** - הכנה סופית לפרסום
  
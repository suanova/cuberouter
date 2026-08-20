/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
/*
 * CubeRouter landing page — TS port of the searouter-isuanova home page.
 * Renders hero (copy + WebGL globe + terminal + stats), features, enterprise,
 * quick-start steps, CTA, and a landing footer. The site header comes from
 * PublicLayout; this component owns everything below it.
 */

import { Link } from '@tanstack/react-router'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import Globe from './globe'
import './landing.css'

// Demo base URL for the API examples: point at the deployment serving this
// page so a copied example talks to the right tenant, not a hard-coded host.
const DEMO_API_BASE =
  typeof window !== 'undefined'
    ? window.location.origin
    : 'https://cube-router.com'

// Syntax-highlighted API call examples (markup matches landing.css .c-* classes).
const CODE_EXAMPLES: Record<string, string> = {
  curl: `<span class="c-prompt">$</span> curl ${DEMO_API_BASE}/v1/chat/completions <span class="c-punct">\\</span>
    -H <span class="c-str">"Authorization: Bearer </span><span class="c-var">$CR_API_KEY</span><span class="c-str">"</span> <span class="c-punct">\\</span>
    -H <span class="c-str">"Content-Type: application/json"</span> <span class="c-punct">\\</span>
    -d <span class="c-str">'{"model":"claude-sonnet-4","messages":[{"role":"user","content":"Hello"}],"stream":true}'</span>`,

  python: `<span class="c-key">from</span> openai <span class="c-key">import</span> OpenAI

client = OpenAI(
    base_url=<span class="c-str">"${DEMO_API_BASE}/v1"</span>,
    api_key=<span class="c-var">"$CR_API_KEY"</span>,
)

stream = client.chat.completions.create(
    model=<span class="c-str">"claude-sonnet-4"</span>,
    messages=[{<span class="c-str">"role"</span>: <span class="c-str">"user"</span>, <span class="c-str">"content"</span>: <span class="c-str">"Hello"</span>}],
    stream=<span class="c-key">True</span>,
)
<span class="c-key">for</span> chunk <span class="c-key">in</span> stream:
    <span class="c-key">print</span>(chunk.choices[<span class="c-num">0</span>].delta.content <span class="c-key">or</span> <span class="c-str">""</span>, end=<span class="c-str">""</span>)`,

  node: `<span class="c-key">import</span> OpenAI <span class="c-key">from</span> <span class="c-str">"openai"</span>;

<span class="c-key">const</span> client = <span class="c-key">new</span> OpenAI({
  baseURL: <span class="c-str">"${DEMO_API_BASE}/v1"</span>,
  apiKey: process.env.<span class="c-var">CR_API_KEY</span>,
});

<span class="c-key">const</span> stream = <span class="c-key">await</span> client.chat.completions.create({
  model: <span class="c-str">"claude-sonnet-4"</span>,
  messages: [{ role: <span class="c-str">"user"</span>, content: <span class="c-str">"Hello"</span> }],
  stream: <span class="c-key">true</span>,
});

<span class="c-key">for await</span> (<span class="c-key">const</span> chunk <span class="c-key">of</span> stream) {
  process.stdout.write(chunk.choices[<span class="c-num">0</span>]?.delta?.content ?? <span class="c-str">""</span>);
}`,
}

const TABS = [
  { id: 'curl', label: 'curl' },
  { id: 'python', label: 'python' },
  { id: 'node', label: 'node' },
]

// Inline SVG icons (lucide style, stroke 1.6).
const IconRoute = () => (
  <svg viewBox='0 0 24 24'>
    <path d='M21 12a9 9 0 1 1-3.15-6.85M21 4v5h-5' />
  </svg>
)
const IconBolt = () => (
  <svg viewBox='0 0 24 24'>
    <path d='M13 2 3 14h8l-1 8 10-12h-8l1-8z' />
  </svg>
)
const IconShield = () => (
  <svg viewBox='0 0 24 24'>
    <path d='M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z' />
  </svg>
)
const IconChart = () => (
  <svg viewBox='0 0 24 24'>
    <path d='M3 3v18h18M7 14l4-4 4 4 5-5' />
  </svg>
)

export function Landing() {
  const { t } = useTranslation()
  const [activeTab, setActiveTab] = useState('curl')

  return (
    <div className='landing-cuberouter'>
      {/* ==================== Hero ==================== */}
      <section className='cr-hero' id='top'>
        <div className='cr-container cr-hero__inner'>
          <div className='cr-hero__grid'>
            <div className='cr-hero__copy'>
              <span className='cr-hero__badge'>
                <span className='cr-hero__badge-dot' aria-hidden='true' />
                {t('Domestic Open-source AI Providers · Unified API')}
              </span>

              <h1 className='cr-hero__title'>
                <span className='cr-hero__title-prefix'>The AI</span>
                <span className='cr-hero__title-brand'>
                  CubeRouter<span className='cr-hero__title-dim'>.</span>
                </span>
              </h1>

              <p className='cr-hero__subtitle'>{t('One interface to access DeepSeek, GLM (Zhipu), MiniMax, Qwen, and Kimi models. Powered by Suanova Technology enterprise computing — better pricing, more stable service, no subscription limits.')}</p>

              <div className='cr-hero__actions'>
                <Link to='/sign-up' className='cr-btn cr-btn--primary cr-btn--lg'>
                  {t('Get API Key')}
                </Link>
                <Link to='/pricing' className='cr-btn cr-btn--ghost cr-btn--lg'>
                  {t('View Pricing')}
                </Link>
              </div>
            </div>

            <div className='cr-hero__visual'>
              <Globe />
            </div>
          </div>

          <div
            className='cr-terminal'
            role='region'
            aria-label={t('API Call Example')}
          >
            <div className='cr-terminal__bar'>
              <span className='cr-terminal__dot' aria-hidden='true' />
              <span className='cr-terminal__dot' aria-hidden='true' />
              <span className='cr-terminal__dot' aria-hidden='true' />
              <div className='cr-terminal__tabs' role='tablist'>
                {TABS.map(({ id, label }) => (
                  <button
                    key={id}
                    type='button'
                    role='tab'
                    aria-selected={activeTab === id}
                    className={`cr-terminal__tab ${activeTab === id ? 'cr-terminal__tab--active' : ''}`}
                    onClick={() => setActiveTab(id)}
                  >
                    {label}
                  </button>
                ))}
              </div>
            </div>
            <div className='cr-terminal__body'>
              <pre dangerouslySetInnerHTML={{ __html: CODE_EXAMPLES[activeTab] }} />
            </div>
          </div>
        </div>

        <div className='cr-stats'>
          <div className='cr-container cr-stats__inner'>
            <div className='cr-stat'>
              <div className='cr-stat__value'>30T+</div>
              <div className='cr-stat__label'>{t('Monthly Tokens')}</div>
            </div>
            <div className='cr-stat'>
              <div className='cr-stat__value'>5M+</div>
              <div className='cr-stat__label'>{t('Global Users')}</div>
            </div>
            <div className='cr-stat'>
              <div className='cr-stat__value'>60+</div>
              <div className='cr-stat__label'>{t('Active Providers')}</div>
            </div>
            <div className='cr-stat'>
              <div className='cr-stat__value'>300+</div>
              <div className='cr-stat__label'>{t('Models Supported')}</div>
            </div>
          </div>
        </div>
      </section>

      {/* ==================== Features ==================== */}
      <section className='cr-section cr-section--no-border' id='features'>
        <div className='cr-container'>
          <div className='cr-section__head'>
            <div className='cr-section__eyebrow'>{t('Features')}</div>
            <h2 className='cr-section__title'>{t('AI Gateway Built for Enterprise')}</h2>
            <p className='cr-section__desc'>{t('Unified access, intelligent routing, and fine-grained control — making AI a truly manageable infrastructure.')}</p>
          </div>

          <div className='cr-features'>
            <article className='cr-feature'>
              <div className='cr-feature__icon' aria-hidden='true'>
                <IconRoute />
              </div>
              <h3 className='cr-feature__title'>{t('Smart Routing')}</h3>
              <p className='cr-feature__desc'>{t('Intelligently select the best model and channel with load balancing and failover for stable, efficient service.')}</p>
            </article>

            <article className='cr-feature'>
              <div className='cr-feature__icon' aria-hidden='true'>
                <IconBolt />
              </div>
              <h3 className='cr-feature__title'>{t('Low Latency & High Performance')}</h3>
              <p className='cr-feature__desc'>{t('Powered by Suanova Technology global infrastructure with edge deployment, minimal first-token latency, and optimal throughput.')}</p>
            </article>

            <article className='cr-feature'>
              <div className='cr-feature__icon' aria-hidden='true'>
                <IconShield />
              </div>
              <h3 className='cr-feature__title'>{t('Enterprise Security')}</h3>
              <p className='cr-feature__desc'>{t('Fine-grained data policies, private deployment, and data sovereignty protection for compliance and auditing.')}</p>
            </article>

            <article className='cr-feature'>
              <div className='cr-feature__icon' aria-hidden='true'>
                <IconChart />
              </div>
              <h3 className='cr-feature__title'>{t('Fine-grained Control')}</h3>
              <p className='cr-feature__desc'>{t('Quota management, traffic control, multi-dimensional statistics and billing to help enterprises track real costs.')}</p>
            </article>
          </div>
        </div>
      </section>

      {/* ==================== Enterprise ==================== */}
      <section className='cr-section' id='enterprise'>
        <div className='cr-container'>
          <div className='cr-enterprise'>
            <div>
              <div className='cr-section__eyebrow'>{t('Enterprise')}</div>
              <h2 className='cr-section__title' style={{ maxWidth: '12ch' }}>
                {t('Built for Large Organizations')}
              </h2>
              <p className='cr-section__desc'>{t('Enterprise edition provides auditing, private deployment, and customized SLAs, covering security, compliance, and scalability needs.')}</p>
              <div className='cr-enterprise__actions'>
                <a
                  href='mailto:cube-router.sales@isuanova.com'
                  className='cr-btn cr-btn--primary'
                >
                  {t('Contact Sales')}
                </a>
                <Link to='/pricing' className='cr-btn cr-btn--ghost'>
                  {t('View Pricing')}
                </Link>
              </div>
            </div>

            <div className='cr-enterprise__items'>
              <div className='cr-enterprise-item'>
                <span className='cr-enterprise-item__num'>01</span>
                <div>
                  <h3 className='cr-enterprise-item__title'>
                    {t('Enterprise Security')}
                  </h3>
                  <p className='cr-enterprise-item__desc'>
                    {t('Fine-grained access control, complete audit logs, and support for on-premise deployment.')}
                  </p>
                </div>
              </div>
              <div className='cr-enterprise-item'>
                <span className='cr-enterprise-item__num'>02</span>
                <div>
                  <h3 className='cr-enterprise-item__title'>
                    {t('Advanced Analytics')}
                  </h3>
                  <p className='cr-enterprise-item__desc'>
                    {t('Usage analytics, cost optimization recommendations, custom reports, and multi-team billing statistics.')}
                  </p>
                </div>
              </div>
              <div className='cr-enterprise-item'>
                <span className='cr-enterprise-item__num'>03</span>
                <div>
                  <h3 className='cr-enterprise-item__title'>
                    {t('Dedicated Support')}
                  </h3>
                  <p className='cr-enterprise-item__desc'>
                    {t('Dedicated technical support, customer success manager, custom feature development, and priority SLA.')}
                  </p>
                </div>
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* ==================== Quick Start ==================== */}
      <section className='cr-section cr-section--subtle'>
        <div className='cr-container'>
          <div className='cr-section__head'>
            <div className='cr-section__eyebrow'>{t('Quick Start')}</div>
            <h2 className='cr-section__title'>{t('Get Started in 3 Steps')}</h2>
            <p className='cr-section__desc'>{t('Fully compatible with OpenAI API format. From registration to first API call in just minutes.')}</p>
          </div>

          <div className='cr-steps'>
            <div className='cr-step'>
              <div className='cr-step__num'>{t('01 / Register')}</div>
              <h3 className='cr-step__title'>{t('Create Account')}</h3>
              <p className='cr-step__desc'>{t('Register a CubeRouter account and get your dedicated API access.')}</p>
            </div>
            <div className='cr-step'>
              <div className='cr-step__num'>{t('02 / Top Up')}</div>
              <h3 className='cr-step__title'>{t('Buy Credits')}</h3>
              <p className='cr-step__desc'>{t('Top up credits on demand — flexible pay-as-you-go billing, scale anytime.')}</p>
            </div>
            <div className='cr-step'>
              <div className='cr-step__num'>{t('03 / Connect')}</div>
              <h3 className='cr-step__title'>{t('Get API Key')}</h3>
              <p className='cr-step__desc'>{t('Create an API Key, replace the base_url, and start calling all supported models.')}</p>
            </div>
          </div>
        </div>
      </section>

      {/* ==================== CTA ==================== */}
      <section className='cr-section'>
        <div className='cr-container cr-cta__inner'>
          <div className='cr-section__eyebrow'>{t('Get Started')}</div>
          <h2 className='cr-cta__title'>{t('Start now, integrate AI into your business.')}</h2>
          <p className='cr-cta__desc'>{t('Professional, stable, and efficient AI gateway to help your team build AI applications faster.')}</p>
          <div className='cr-cta__actions'>
            <Link to='/sign-up' className='cr-btn cr-btn--primary cr-btn--lg'>
              {t('Free Trial')}
            </Link>
            <a
              href='mailto:cube-router.cs-support@isuanova.com'
              className='cr-btn cr-btn--ghost cr-btn--lg'
            >
              {t('Contact Us')}
            </a>
          </div>
        </div>
      </section>

      {/* ==================== Footer ==================== */}
      <footer className='cr-footer' id='contact'>
        <div className='cr-container'>
          <div className='cr-footer__grid'>
            <div>
              <div className='cr-footer__brand'>
                <img src='/head.png' alt='CubeRouter' className='dark:brightness-0 dark:invert' />
              </div>
              <p className='cr-footer__desc'>{t('oneSuanova is a leading AI service provider. Through its proprietary Token-as-a-Service (TaaS) platform, it provides stable and efficient AI Token cloud services for enterprises and institutions across industries. The brand is wholly owned and operated by Suanova Technology, dedicated to building high-standard, scalable next-generation AI computing infrastructure.')}</p>
            </div>

            <div>
              <h4 className='cr-footer__col-title'>{t('Product')}</h4>
              <ul className='cr-footer__links'>
                <li>
                  <a href='#features' className='cr-footer__link'>
                    {t('Features')}
                  </a>
                </li>
                <li>
                  <Link to='/pricing' className='cr-footer__link'>
                    {t('Models')}
                  </Link>
                </li>
                <li>
                  <Link to='/pricing' className='cr-footer__link'>
                    {t('Pricing')}
                  </Link>
                </li>
              </ul>
            </div>

            <div>
              <h4 className='cr-footer__col-title'>{t('Enterprise')}</h4>
              <ul className='cr-footer__links'>
                <li>
                  <a href='#enterprise' className='cr-footer__link'>
                    {t('Enterprise Edition')}
                  </a>
                </li>
                <li>
                  <a
                    href='mailto:cube-router.sales@isuanova.com'
                    className='cr-footer__link'
                  >
                    {t('Contact Sales')}
                  </a>
                </li>
              </ul>
            </div>

            <div>
              <h4 className='cr-footer__col-title'>{t('Legal')}</h4>
              <ul className='cr-footer__links'>
                <li>
                  <Link to='/privacy-policy' className='cr-footer__link'>
                    {t('Privacy Policy')}
                  </Link>
                </li>
                <li>
                  <Link to='/user-agreement' className='cr-footer__link'>
                    {t('Terms of Service')}
                  </Link>
                </li>
              </ul>
            </div>
          </div>

          <div className='cr-footer__bottom'>
            {/* Single consolidated copyright. The project is AGPL-3.0, so the
                legacy "All Rights Reserved" phrase (which contradicts the
                rights the license grants) is intentionally omitted; upstream
                credits live on the About page. */}
            <span>{t('© 2026 CubeRouter · Suanova Technology Ltd')}</span>
          </div>
        </div>
      </footer>
    </div>
  )
}

import type { ReactNode } from "react";
import clsx from "clsx";
import Link from "@docusaurus/Link";
import useDocusaurusContext from "@docusaurus/useDocusaurusContext";
import Layout from "@theme/Layout";
import HomepageFeatures from "@site/src/components/HomepageFeatures";
import Heading from "@theme/Heading";

import styles from "./index.module.css";

function HomepageHeader() {
  const { siteConfig } = useDocusaurusContext();
  return (
    <header className={clsx("hero hero--primary", styles.heroBanner)}>
      <div className="container">
        <Heading as="h1" className="hero__title">
          {siteConfig.title}
        </Heading>
        <p className="hero__subtitle">{siteConfig.tagline}</p>
        <p className={styles.heroDescription}>
          A Kubernetes operator for monitoring CronJobs with SLA tracking,
          intelligent alerting, and a built-in dashboard.
        </p>
        <div className={styles.buttons}>
          <Link
            className="button button--secondary button--lg"
            to="/docs/getting-started/installation"
          >
            Get Started
          </Link>
          <Link
            className="button button--outline button--lg"
            to="/docs/getting-started/introduction"
          >
            Learn More
          </Link>
        </div>
      </div>
    </header>
  );
}

function ScreenshotSection() {
  return (
    <section className={styles.screenshotSection}>
      <div className="container">
        <div className="row">
          <div className="col col--12">
            <Heading as="h2" className="text--center margin-bottom--lg">
              Visualize Your CronJob Health
            </Heading>
          </div>
        </div>
        <div className="row">
          <div className="col col--12">
            <div className={styles.screenshotWrapper}>
              <img
                src="/cronjob-guardian/img/screenshots/dashboard.png"
                alt="CronJob Guardian Dashboard"
                className={styles.screenshot}
              />
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}

export default function Home(): ReactNode {
  const { siteConfig } = useDocusaurusContext();
  return (
    <Layout
      title="Kubernetes CronJob Monitoring"
      description="A Kubernetes operator for monitoring CronJobs with SLA tracking, intelligent alerting, and a built-in dashboard."
    >
      <HomepageHeader />
      <main>
        <HomepageFeatures />
        <ScreenshotSection />
      </main>
    </Layout>
  );
}

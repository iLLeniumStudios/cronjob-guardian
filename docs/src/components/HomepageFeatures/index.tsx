import type { ReactNode } from "react";
import clsx from "clsx";
import Heading from "@theme/Heading";
import styles from "./styles.module.css";

type FeatureItem = {
  title: string;
  icon: string;
  description: ReactNode;
};

const FeatureList: FeatureItem[] = [
  {
    title: "Dead-Man's Switch",
    icon: "🔔",
    description: (
      <>
        Detect when CronJobs stop running entirely. Auto-detects expected
        intervals from cron schedules and alerts when jobs are missed.
      </>
    ),
  },
  {
    title: "SLA Tracking",
    icon: "📊",
    description: (
      <>
        Monitor success rates and duration percentiles (P50/P95/P99). Get
        alerted when success rates drop below your defined thresholds.
      </>
    ),
  },
  {
    title: "Smart Alerts",
    icon: "🎯",
    description: (
      <>
        Receive rich alerts with pod logs, Kubernetes events, and intelligent
        fix suggestions. Route critical and warning alerts to different
        channels.
      </>
    ),
  },
  {
    title: "Built-in Dashboard",
    icon: "📈",
    description: (
      <>
        Visualize health with success rate charts, duration trends, and
        calendar heatmaps. Export reports as CSV or PDF.
      </>
    ),
  },
];

function Feature({ title, icon, description }: FeatureItem) {
  return (
    <div className={clsx("col col--3")}>
      <div className="text--center">
        <span className={styles.featureIcon}>{icon}</span>
      </div>
      <div className="text--center padding-horiz--md">
        <Heading as="h3">{title}</Heading>
        <p>{description}</p>
      </div>
    </div>
  );
}

export default function HomepageFeatures(): ReactNode {
  return (
    <section className={styles.features}>
      <div className="container">
        <div className="row">
          {FeatureList.map((props, idx) => (
            <Feature key={idx} {...props} />
          ))}
        </div>
      </div>
    </section>
  );
}

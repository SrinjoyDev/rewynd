// Background worker: drains the "email.send" queue and sends each email.
import { mailer } from "./mailer";

export async function processJob(job) {
  // no request-scoped logging here; failures are easy to miss
  if (job.type === "email.send") {
    await mailer.sendMail(job.to, job.subject, job.body);
  }
}

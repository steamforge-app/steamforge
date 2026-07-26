const MS_PER_DAY = 1000 * 60 * 60 * 24;
const DAYS_PER_MONTH = 30;
const DAYS_PER_YEAR = 365;

export function formatLastPlayed(unixTimestamp: number): string {
  if (!unixTimestamp) return '';
  const playedDate = new Date(unixTimestamp * 1000);
  const daysSincePlayed = Math.floor((Date.now() - playedDate.getTime()) / MS_PER_DAY);
  if (daysSincePlayed === 0) return 'Today';
  if (daysSincePlayed === 1) return 'Yesterday';
  if (daysSincePlayed < DAYS_PER_MONTH) return `${daysSincePlayed}d ago`;
  if (daysSincePlayed < DAYS_PER_YEAR) return `${Math.floor(daysSincePlayed / DAYS_PER_MONTH)}mo ago`;
  return `${Math.floor(daysSincePlayed / DAYS_PER_YEAR)}y ago`;
}

export function formatHoursMinutes(hours: number): string {
  const totalMinutes = Math.round(hours * 60);
  const wholeHours = Math.floor(totalMinutes / 60);
  const minutes = totalMinutes % 60;
  if (wholeHours === 0) return `${minutes}m`;
  if (minutes === 0) return `${wholeHours}h`;
  return `${wholeHours}h ${minutes}m`;
}

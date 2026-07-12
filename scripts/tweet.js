const { TwitterApi } = require('twitter-api-v2');
const fs = require('fs');
const path = require('path');

// X APIクライアント初期化
const client = new TwitterApi({
  appKey: process.env.X_API_KEY,
  appSecret: process.env.X_API_KEY_SECRET,
  accessToken: process.env.X_ACCESS_TOKEN,
  accessSecret: process.env.X_ACCESS_TOKEN_SECRET,
});

const tweetsDir = './tweets';
const postedDir = './tweets/posted';

async function main() {
  // tweets/フォルダがなければスキップ
  if (!fs.existsSync(tweetsDir)) {
    console.log('No tweets/ directory found. Skipping.');
    return;
  }

  // posted/フォルダがなければ作成
  if (!fs.existsSync(postedDir)) {
    fs.mkdirSync(postedDir, { recursive: true });
  }

  // tweets/フォルダのJSONファイルを取得
  const files = fs.readdirSync(tweetsDir)
    .filter(f => f.endsWith('.json') && !f.startsWith('.'));

  const now = new Date();

  for (const file of files) {
    const filePath = path.join(tweetsDir, file);
    const stat = fs.statSync(filePath);
    
    // ディレクトリはスキップ
    if (stat.isDirectory()) continue;

    const data = JSON.parse(fs.readFileSync(filePath, 'utf-8'));

    // scheduled_atがある場合、時刻チェック
    if (data.scheduled_at) {
      const scheduledTime = new Date(data.scheduled_at);
      if (scheduledTime > now) {
        console.log(`⏰ Skipped (scheduled for later): ${file}`);
        continue;
      }
    }

    try {
      // スレッドの場合
      if (data.thread && Array.isArray(data.thread)) {
        let lastTweetId = null;
        for (const tweet of data.thread) {
          const result = await client.v2.tweet({
            text: tweet.text,
            ...(lastTweetId && { reply: { in_reply_to_tweet_id: lastTweetId } }),
          });
          lastTweetId = result.data.id;
          console.log(`✅ Posted thread part: ${result.data.id}`);
        }
      } else {
        // 単一ツイート
        const result = await client.v2.tweet({ text: data.text });
        console.log(`✅ Posted: ${result.data.id} - ${file}`);
      }

      // 投稿済みファイルを移動
      const postedPath = path.join(postedDir, file);
      fs.renameSync(filePath, postedPath);
      console.log(`📁 Moved to posted/: ${file}`);

    } catch (error) {
      console.error(`❌ Failed: ${file}`, error.message);
      // エラーでも続行（他のツイートを投稿）
    }
  }
}

main().catch(console.error);

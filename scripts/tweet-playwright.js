#!/usr/bin/env node
/**
 * Playwright版 X(Twitter)自動投稿スクリプト
 * 
 * 使用方法:
 *   node tweet-playwright.js "投稿内容"
 *   node tweet-playwright.js --file tweet.txt
 * 
 * 環境変数:
 *   PLAYWRIGHT_STORAGE_STATE: Cookie保存ファイルパス（デフォルト: ./x-cookies.json）
 * 
 * 必要条件:
 *   - Node.js 18+
 *   - playwright インストール済み（npx playwright install chromium）
 *   - 初回はブラウザでログインしてCookieを保存する必要あり
 */

const { chromium } = require('playwright');
const fs = require('fs');
const path = require('path');

const STORAGE_STATE_PATH = process.env.PLAYWRIGHT_STORAGE_STATE || './x-cookies.json';
const X_URL = 'https://x.com';

async function postTweet(content, headless = false) {
  console.log('Playwright版X投稿を開始します...');
  
  // ブラウザ起動
  const browser = await chromium.launch({
    headless: headless, // ローカルでは画面表示、CI環境ではheadless
  });
  
  let context;
  
  // Cookie状態があれば復元
  if (fs.existsSync(STORAGE_STATE_PATH)) {
    console.log('保存済みCookieを読み込み中...');
    context = await browser.newContext({
      storageState: STORAGE_STATE_PATH,
    });
  } else {
    console.log('Cookieが見つかりません。新規セッションを開始...');
    context = await browser.newContext();
  }
  
  const page = await context.newPage();
  
  try {
    // X.comにアクセス
    console.log('X.comにアクセス中...');
    await page.goto(`${X_URL}/home`, { waitUntil: 'domcontentloaded', timeout: 30000 });
    
    // 少し待機してページ読み込みを待つ
    await page.waitForTimeout(3000);
    
    // ログイン状態確認
    console.log('ログイン状態を確認中...');
    const isLoggedIn = await page.locator('[data-testid="SideNav_NewTweet_Button"]').isVisible({ timeout: 10000 }).catch(() => false);
    
    if (!isLoggedIn) {
      throw new Error('ログインが必要です。--login オプションで手動ログインしてください。');
    }
    
    console.log('ログイン確認OK');
    
    // 投稿ボタンをクリック
    console.log('投稿フォームを開きます...');
    await page.locator('[data-testid="SideNav_NewTweet_Button"]').click();
    
    // 投稿テキストエリアを待機（モーダル内のものを使用）
    await page.waitForSelector('[data-testid="tweetTextarea_0"]', { timeout: 10000 });
    
    // 投稿内容を入力（最初に見つかったものを使用）
    console.log('投稿内容を入力中...');
    await page.locator('[data-testid="tweetTextarea_0"]').first().fill(content);
    
    // 少し待機
    await page.waitForTimeout(1000);
    
    // 投稿ボタンをクリック（モーダル内のもの）
    console.log('投稿を送信中...');
    await page.locator('[data-testid="tweetButton"]').first().click();
    
    // 投稿完了を待機
    await page.waitForTimeout(3000);
    
    console.log('✅ 投稿が完了しました！');
    console.log(`内容: ${content.substring(0, 50)}${content.length > 50 ? '...' : ''}`);
    
    // Cookie状態を保存
    await context.storageState({ path: STORAGE_STATE_PATH });
    console.log('Cookieを保存しました。');
    
  } catch (error) {
    console.error('❌ 投稿に失敗しました:', error.message);
    process.exit(1);
  } finally {
    await browser.close();
  }
}

async function login() {
  console.log('手動ログインモードを開始します...');
  
  const browser = await chromium.launch({
    headless: false, // ログイン時は画面表示
  });
  
  const context = await browser.newContext();
  const page = await context.newPage();
  
  console.log('X.comのログインページを開きます...');
  await page.goto(`${X_URL}/login`);
  
  console.log('\n========================================');
  console.log('ブラウザでログインしてください。');
  console.log('ログイン完了後、ホーム画面が表示されたら');
  console.log('このターミナルでEnterを押してください。');
  console.log('========================================\n');
  
  // ユーザー入力を待機
  await new Promise(resolve => {
    process.stdin.once('data', resolve);
  });
  
  // Cookie状態を保存
  await context.storageState({ path: STORAGE_STATE_PATH });
  console.log(`✅ Cookieを保存しました: ${STORAGE_STATE_PATH}`);
  
  await browser.close();
}

async function main() {
  const args = process.argv.slice(2);
  
  if (args.length === 0) {
    console.log('使用方法:');
    console.log('  node tweet-playwright.js "投稿内容"');
    console.log('  node tweet-playwright.js --file tweet.txt');
    console.log('  node tweet-playwright.js --login    # 初回ログイン用');
    process.exit(1);
  }
  
  // ログインモード
  if (args[0] === '--login') {
    await login();
    return;
  }
  
  // ファイルから読み込み
  let content;
  if (args[0] === '--file') {
    const filePath = args[1];
    if (!filePath) {
      console.error('ファイルパスを指定してください');
      process.exit(1);
    }
    content = fs.readFileSync(filePath, 'utf-8').trim();
  } else {
    content = args.join(' ');
  }
  
  if (!content) {
    console.error('投稿内容が空です');
    process.exit(1);
  }
  
  await postTweet(content, process.env.CI === 'true');
}

main().catch(console.error);

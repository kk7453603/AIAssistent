# PAA Future Improvements Roadmap

## Уровень 1: UX и взаимодействие
- [ ] Multi-modal input (vision models: LLaVA/Qwen-VL, фото → описание → vault)
- [ ] Голосовой ввод/вывод (Whisper STT → агент → TTS)
- [ ] Telegram бот (полноценный интерфейс к агенту)
- [ ] Proactive notifications (напоминания о задачах через Telegram/webhook)
- [ ] **Custom UI на Tauri** (замена OpenWebUI)

## Уровень 2: Качество и память
- [ ] RAG evaluation pipeline (RAGAS, faithfulness, relevancy)
- [ ] Adaptive model routing (автовыбор модели по типу задачи)
- [ ] Knowledge graph (граф связей между заметками)
- [ ] Auto-tagging & classification (автотегирование при синхронизации)

## Уровень 3: Автоматизация и агенты
- [ ] Multi-agent orchestration (researcher, coder, writer)
- [ ] Scheduled tasks (recurring cron-подобный scheduler)
- [ ] Self-improving agent (анализ ошибок → предложение улучшений)
- [ ] Plugin system (добавление тулов через YAML/JSON без Go кода)

## Уровень 4: Инфраструктура
- [ ] Fine-tuning pipeline (LoRA/QLoRA на своих данных)
- [ ] Vector DB optimization (ColBERT, multi-vector)
- [ ] Edge deployment (мобильный агент через ONNX/llama.cpp)

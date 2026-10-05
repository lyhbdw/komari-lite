import { resolveI18nText } from "@/utils/i18nText";

// 法律声明与合规指引全文（保留中文与英文兜底）
const EULAS: Record<string, string> = {
  "zh_CN": `法律声明与合规指引

重要提示
  本文档旨在明确 Komari（“本软件”）的合法使用边界、用户权利义务与风险提示。请在下载、安装或使用本软件前，务必完整阅读并理解本声明的全部内容。一经下载、安装或使用，即视为您已理解并同意受本声明约束。

1. 适用范围
  本声明适用于所有直接或间接获取、安装、访问或使用本软件及其衍生工具、文档与服务的自然人、法人或其他组织。

2. 定义
  - “您/用户”：指任何以任何方式使用本软件的主体。
  - “本软件/Komari”：指开源发布的 Komari Monitor 及相关组件、示例、脚本与文档。
  - “目标设备”：指被您管理或操作的任何主机、服务器、虚拟机、容器或网络设备。

3. 许可与使用边界
  - 本软件依据开源许可证（MIT）授权使用。除 MIT 许可另有规定外，本声明作为合规与风险提示的补充条款。
  - 本软件仅用于合法、合规、经授权的系统管理、监测和研究目的。不得将本软件用于任何违反法律法规、监管政策或第三方合法权益之行为。
  - 本软件不提供、亦不包含任何绕过安全控制、破解或渗透测试的隐含授权。进行安全测试前，您必须取得被测系统的事先明确书面授权。

4. 使用者责任（极其重要）
  您对通过本软件进行的所有操作承担全部且唯一的法律责任。无论您的使用意图为何，只要行为发生，所产生的一切后果均由您自行承担，与本软件开发者无关。开发者不对您的使用方式进行审查、监控或背书，也不承担任何直接、间接或连带责任。

5. 严禁行为（包括但不限于）
  5.1 未经授权的入侵、控制或访问任何计算机、服务器、网络或账户。
  5.2 扫描端口、探测漏洞或从事任何未经授权的安全测试与信息收集。
  5.3 发起或参与网络攻击（如 DoS/DDoS、流量劫持、嗅探、中间人攻击等）。
  5.4 安装、分发或执行恶意代码、木马、后门、勒索软件或僵尸网络程序；利用被控设备进行挖矿、垃圾邮件发送、钓鱼或诈骗。
  5.5 未经授权收集、处理、篡改、删除或泄露个人信息、商业秘密、知识产权或其他敏感数据；进行屏幕窥视、键盘记录、文件监听等监控行为。
  5.6 传播任何违法或侵权内容，包括但不限于淫秽、暴力、恐怖主义、仇恨、歧视、诽谤或其他不当内容。
  5.7 规避技术保护措施、逆向工程（法律允许范围除外）、干扰/破坏他人系统稳定性与可用性。

6. 合规与地区要求（特别说明）
  您必须确保您的使用行为同时符合：
  - 您所在地法律法规；
  - 目标设备所在地法律法规；
  - 相关行业监管规则与自律规范。
  特别地，若您位于中国大陆地区，须严格遵守《网络安全法》《数据安全法》《个人信息保护法》《关键信息基础设施安全保护条例》等相关法律法规与标准。若您位于香港特别行政区、澳门特别行政区及台湾地区，亦须遵守当地适用之法律法规。任何违法或违规风险由您自行承担。

7. 数据与隐私合规
  - 处理任何个人信息前，您应具备合法、正当、必要之处理依据，并履行告知、同意、最小化、目的限制、安全保障与跨境合规等义务。
  - 若涉及敏感数据或机密信息，您应实施加密、脱敏、访问控制、日志审计等合理、必要的技术与管理措施。
  - 为符合法律合规、安全审计、防滥用与争议处理之目的，在适用法律允许且必要时，我们可能会收集并保留最小化的网络元数据。该数据仅用于安全与法务合规用途，除法律纠纷外，我们不会主动分享您的数据。
  - 数据安全与加密：我们在合理可行范围内采取行业通行的安全措施，以降低数据泄露、篡改与丢失风险。
  - 除非基于法律法规要求、执法或监管部门的合法请求，或为保护我们及用户的合法权益所必需，我们不会向第三方披露前述数据。

8. 安全、内容与知识产权合规
  - 您应确保对目标设备具有合法的管理权限或操作授权。
  - 不得利用本软件侵犯任何第三方的知识产权、名誉权、隐私权或其他合法权益。
  - 对于由您接入或展示的内容，您独立承担合规与版权责任。

9. 第三方组件与服务
  本软件可能依赖第三方开源库或外部服务。该等第三方的可用性、准确性与合规性由其各自提供方负责。您应遵循相应许可或使用条款，并自行评估与承担相关风险。

10. 风险提示
  - 本软件以“现状”提供，可能受限于网络、硬件、系统差异而产生不兼容、不可用或误用风险。
  - 远程控制与批量操作具有潜在高风险，请务必采取最小权限、分级授权、多因素认证、审计留痕、分环境验证等最佳实践。

11. 免责声明
  在适用法律允许的最大范围内：本软件及其开发者不对本软件的适用性、稳定性、正确性、可用性或特定目的适配性作出任何明示或默示保证；亦不对因使用或无法使用本软件而导致的任何形式的损失或损害承担责任。

12. 责任限制
  在任何情况下，本软件开发者对您因本软件产生或与之相关的任何间接、偶然、特殊、惩罚性或后果性损害不承担责任；对任何直接损失的总责任（如有）以适用法律允许的最低上限为准。

13. 赔偿条款
  若您因违反本声明或适用法律而引发任何第三方主张、索赔、纠纷或处罚，您应独立承担全部责任，并使本软件开发者及其贡献者免受损害。

14. 终止与技术支持
  对于任何涉嫌或实际违反本声明的用户，开发者有权拒绝或终止提供任何形式的技术支持或协助。

15. 出口管制与制裁合规
  您承诺遵守适用的出口管制、再出口与经济制裁法律法规，不得将本软件用于或提供给受限制的国家、地区、实体或个人。

16. 通知与修订
  本声明可能随版本更新或法律政策变化进行修订。更新后的版本将以适当方式公布并自公布之日起生效。

17. 适用法律与争议解决
  在不抵触强制性法律的前提下，本声明的解释与适用以本软件开源仓库维护者所在地法律为准；争议应友好协商解决，协商不成的，提交有管辖权的法院或仲裁机构处理。

18. 最终条款
  若您不同意本声明任何内容，或无法确保您的使用完全合法合规，请立即停止使用并卸载本软件。继续使用即视为您已阅读、理解并同意本声明全部内容。

生效日期：2025-10-20
`,
  "en": `Legal Notice and Compliance Guide

Important Notice
  This document clarifies the lawful boundaries of using Komari ("the Software"), your rights and obligations, and risk disclosures. Before downloading, installing, or using the Software, please read and understand this notice in full. By downloading, installing, or using the Software, you acknowledge that you have read, understood, and agreed to be bound by this notice.

1. Scope of Application
  This notice applies to all individuals, legal persons, or other organizations that directly or indirectly obtain, install, access, or use the Software and its derivative tools, documentation, and services.

2. Definitions
  - "You/User": any party that uses the Software in any manner.
  - "Software/Komari": Komari Monitor, released as open source, and its related components, examples, scripts, and documentation.
  - "Target Device": any host, server, virtual machine, container, or network device that you manage or operate.

3. License and Usage Boundaries
  - The Software is licensed under the MIT open-source license. Except as otherwise provided by the MIT license, this notice serves as supplementary terms for compliance and risk disclosure.
  - The Software is intended solely for lawful, compliant, and authorized system administration, monitoring, and research purposes. You must not use the Software for any activity that violates laws, regulations, regulatory policies, or the legitimate rights of third parties.
  - The Software does not provide or imply any authorization to bypass security controls, crack, or perform penetration testing. Before conducting security testing, you must obtain prior explicit written authorization from the owner of the system being tested.

4. User Responsibility (Extremely Important)
  You bear full and sole legal responsibility for all operations performed through the Software. Regardless of your intent, all consequences arising from such actions are your sole responsibility and are not attributable to the developers of the Software. The developers do not review, monitor, or endorse how you use the Software and assume no direct, indirect, or joint liability.

5. Prohibited Conduct (Including but Not Limited To)
  5.1 Unauthorized intrusion, control, or access to any computer, server, network, or account.
  5.2 Port scanning, vulnerability probing, or any unauthorized security testing and information gathering.
  5.3 Initiating or participating in cyberattacks (e.g., DoS/DDoS, traffic hijacking, sniffing, man-in-the-middle attacks).
  5.4 Installing, distributing, or executing malicious code, trojans, backdoors, ransomware, or botnet programs; using controlled devices for cryptocurrency mining, spam, phishing, or fraud.
  5.5 Unauthorized collection, processing, tampering, deletion, or disclosure of personal information, trade secrets, intellectual property, or other sensitive data; conducting monitoring such as screen surveillance, keystroke logging, or file interception.
  5.6 Disseminating any illegal or infringing content, including but not limited to obscene, violent, terrorist, hateful, discriminatory, defamatory, or other inappropriate content.
  5.7 Circumventing technical protection measures, reverse engineering (except as permitted by law), or interfering with/destroying the stability and availability of others' systems.

6. Compliance and Regional Requirements (Special Note)
  You must ensure that your use complies with:
  - the laws and regulations of your location;
  - the laws and regulations of the location of the target devices;
  - relevant industry regulatory rules and self-regulatory standards.
  In particular, if you are located in mainland China, you must strictly comply with the Cybersecurity Law, the Data Security Law, the Personal Information Protection Law, the Regulations on the Security Protection of Critical Information Infrastructure, and other relevant laws, regulations, and standards. If you are located in the Hong Kong Special Administrative Region, the Macao Special Administrative Region, or the Taiwan region, you must also comply with applicable local laws and regulations. You bear all risks arising from any illegal or non-compliant conduct.

7. Data and Privacy Compliance
  - Before processing any personal information, you must have a lawful, legitimate, and necessary basis for processing and fulfill obligations such as notice, consent, minimization, purpose limitation, security safeguards, and cross-border compliance.
  - If sensitive or confidential data is involved, you must implement reasonable and necessary technical and administrative measures such as encryption, desensitization, access control, and log auditing.
  - For purposes of legal compliance, security auditing, abuse prevention, and dispute resolution, and where permitted and necessary under applicable law, we may collect and retain minimal network metadata. Such data is used solely for security and legal compliance purposes, and we will not proactively share your data except in the context of legal disputes.
  - Data security and encryption: we adopt industry-standard security measures to the extent reasonably feasible to reduce the risk of data breaches, tampering, and loss.
  - We will not disclose the aforementioned data to third parties unless required by laws and regulations, legitimate requests from law enforcement or regulators, or as necessary to protect the legitimate rights and interests of us or our users.

8. Security, Content, and Intellectual Property Compliance
  - You must ensure that you have legitimate administrative permissions or operational authorization for target devices.
  - You must not use the Software to infringe any third party's intellectual property, reputation, privacy, or other legitimate rights.
  - For content that you integrate or display, you bear sole responsibility for compliance and copyright.

9. Third-Party Components and Services
  The Software may rely on third-party open-source libraries or external services. The availability, accuracy, and compliance of such third parties are the responsibility of their respective providers. You should follow the corresponding licenses or terms of use and evaluate and assume the associated risks yourself.

10. Risk Disclosure
  - The Software is provided "as is" and may be subject to incompatibility, unavailability, or misuse risks due to differences in networks, hardware, or systems.
  - Remote control and batch operations carry potentially high risks. Please be sure to adopt best practices such as least privilege, tiered authorization, multi-factor authentication, audit trails, and staged environment validation.

11. Disclaimer of Warranties
  To the maximum extent permitted by applicable law, the Software and its developers make no express or implied warranties regarding the suitability, stability, correctness, availability, or fitness for a particular purpose of the Software, and assume no liability for any loss or damage arising from the use of or inability to use the Software.

12. Limitation of Liability
  In no event shall the developers of the Software be liable for any indirect, incidental, special, punitive, or consequential damages arising out of or in connection with the Software; the total liability (if any) for any direct damages shall be limited to the lowest cap permitted by applicable law.

13. Indemnification
  If you give rise to any third-party claim, lawsuit, dispute, or penalty due to your violation of this notice or applicable law, you shall bear full responsibility independently and hold the developers and contributors of the Software harmless.

14. Termination and Technical Support
  The developers reserve the right to refuse or terminate any form of technical support or assistance to users who are suspected of or actually violate this notice.

15. Export Control and Sanctions Compliance
  You undertake to comply with applicable export control, re-export, and economic sanctions laws and regulations and must not use or provide the Software to or for restricted countries, regions, entities, or individuals.

16. Notice and Revisions
  This notice may be revised as versions are updated or laws and policies change. The updated version will be published in an appropriate manner and takes effect upon publication.

17. Governing Law and Dispute Resolution
  Without prejudice to mandatory laws, the interpretation and application of this notice shall be governed by the laws of the jurisdiction where the maintainers of the open-source repository of the Software are located; disputes shall first be resolved through friendly negotiation, and if negotiation fails, they shall be submitted to a court or arbitration institution with jurisdiction.

18. Final Terms
  If you do not agree with any content of this notice, or cannot ensure that your use is fully lawful and compliant, please immediately stop using and uninstall the Software. Continued use is deemed to be your acknowledgment that you have read, understood, and agreed to all the contents of this notice.

Effective date: 2025-10-20
`,
};

export function getEula(language: string): string {
  return resolveI18nText(EULAS, language) ?? EULAS.zh_CN;
}
